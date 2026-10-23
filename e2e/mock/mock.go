// Package mock serves an API from its OpenAPI description, for testing a
// client against the contract without the real service.
//
// It needs no code per endpoint. For each request, it finds the operation,
// validates the request against the description, and answers with the
// examples of the chosen response: its body and its headers. A response
// that does not match the description is reported too, so examples cannot
// drift from their schemas unnoticed.
//
// Requests that fail a security requirement are answered with the
// operation's 401 response. Any other violation is reported, and answered
// with its 400 response.
//
// The Prefer header selects another documented response, as in
// `Prefer: code=429`.
package mock

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/content"
	validatorerrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/paths"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// Server is an [http.Handler] that mocks the API in an OpenAPI description.
type Server struct {
	doc         *v3.Document
	onViolation func(string)

	// The validator is not documented as safe for concurrent use.
	mu        sync.Mutex
	validator validator.Validator
}

// New returns a server for the OpenAPI description spec. onViolation is
// called with a description of each request or response that does not
// match it.
func New(spec []byte, onViolation func(string)) (*Server, error) {
	doc, err := libopenapi.NewDocument(spec)
	if err != nil {
		return nil, fmt.Errorf("parsing the API description: %w", err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("building the API description: %w", err)
	}

	v, errs := validator.NewValidator(doc,
		config.WithURLEncodedBodyValidation(),
		config.WithStandardBodyDecoders(),
		// Later registrations win, so this replaces the stock form decoder.
		config.WithBodyDecoder("application/x-www-form-urlencoded", formDecoder()),
		config.WithAuthenticationFunc(authenticate),
	)
	if len(errs) > 0 {
		return nil, fmt.Errorf("building the validator: %w", errors.Join(errs...))
	}
	if ok, errs := v.ValidateDocument(); !ok {
		return nil, fmt.Errorf("invalid API description: %s", describe(errs))
	}

	return &Server{doc: &model.Model, validator: v, onViolation: onViolation}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	op := s.operation(r)
	if op == nil {
		s.onViolation(fmt.Sprintf("%s %s: no such operation", r.Method, r.URL.Path))
		http.NotFound(w, r)
		return
	}

	s.mu.Lock()
	_, errs := s.validator.ValidateHttpRequest(withBody(r, body))
	s.mu.Unlock()

	var status string
	switch security, others := splitSecurity(errs); {
	case len(others) > 0:
		s.onViolation(fmt.Sprintf("%s %s violates the API description: %s", r.Method, r.URL.Path, describe(others)))
		status = "400"
	case len(security) > 0:
		status = "401"
	default:
		status = preferredStatus(r, op)
	}

	resp := op.Responses.Codes.GetOrZero(status)
	if resp == nil {
		http.Error(w, fmt.Sprintf("the API description has no %s response for %s %s", status, r.Method, r.URL.Path), http.StatusNotImplemented)
		return
	}
	code, err := strconv.Atoi(status)
	if err != nil {
		http.Error(w, fmt.Sprintf("unsupported status %q", status), http.StatusNotImplemented)
		return
	}

	header, payload, err := render(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.checkResponse(withBody(r, body), code, header, payload)

	for k, vs := range header {
		w.Header()[k] = vs
	}
	w.WriteHeader(code)
	_, _ = w.Write(payload)
}

// operation returns the operation the request is for, or nil.
func (s *Server) operation(r *http.Request) *v3.Operation {
	item, _, _ := paths.FindPath(r, s.doc, nil)
	if item == nil {
		return nil
	}
	return item.GetOperations().GetOrZero(strings.ToLower(r.Method))
}

// checkResponse reports a mocked response that does not match the API
// description, which means its examples drifted from its schemas.
func (s *Server) checkResponse(r *http.Request, code int, header http.Header, payload []byte) {
	resp := &http.Response{
		StatusCode: code,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(payload)),
		Request:    r,
	}

	s.mu.Lock()
	_, errs := s.validator.ValidateHttpResponse(r, resp)
	s.mu.Unlock()

	if len(errs) > 0 {
		s.onViolation(fmt.Sprintf("the %d response example of %s %s violates the API description: %s",
			code, r.Method, r.URL.Path, describe(errs)))
	}
}

// preferredStatus returns the status the Prefer header asks for, or else
// the lowest documented success status.
func preferredStatus(r *http.Request, op *v3.Operation) string {
	for pref := range strings.SplitSeq(r.Header.Get("Prefer"), ",") {
		if code, ok := strings.CutPrefix(strings.TrimSpace(pref), "code="); ok {
			return code
		}
	}

	var success []string
	for code := range op.Responses.Codes.KeysFromOldest() {
		if strings.HasPrefix(code, "2") {
			success = append(success, code)
		}
	}
	slices.Sort(success)
	if len(success) == 0 {
		return "200"
	}
	return success[0]
}

// render returns the headers and the body of a response from its examples.
func render(resp *v3.Response) (http.Header, []byte, error) {
	header := http.Header{}
	if resp.Headers != nil {
		for name, h := range resp.Headers.FromOldest() {
			if h == nil || h.Example == nil {
				continue
			}
			var v any
			if err := h.Example.Decode(&v); err != nil {
				return nil, nil, fmt.Errorf("decoding the example of header %s: %w", name, err)
			}
			header.Set(name, fmt.Sprint(v))
		}
	}

	if resp.Content == nil || resp.Content.Len() == 0 {
		return header, nil, nil
	}
	first := resp.Content.First()
	mediaType, content := first.Key(), first.Value()
	header.Set("Content-Type", mediaType)
	if content.Example == nil {
		return header, nil, nil
	}

	var example any
	if err := content.Example.Decode(&example); err != nil {
		return nil, nil, fmt.Errorf("decoding the %s example: %w", mediaType, err)
	}
	if s, ok := example.(string); ok && !strings.Contains(mediaType, "json") {
		return header, []byte(s), nil
	}
	payload, err := json.Marshal(example)
	if err != nil {
		return nil, nil, fmt.Errorf("encoding the %s example: %w", mediaType, err)
	}
	return header, payload, nil
}

// splitSecurity separates the errors of security requirements from the
// others.
func splitSecurity(errs []*validatorerrors.ValidationError) (security, others []*validatorerrors.ValidationError) {
	for _, e := range errs {
		if e.ValidationType == "security" {
			security = append(security, e)
		} else {
			others = append(others, e)
		}
	}
	return security, others
}

func describe(errs []*validatorerrors.ValidationError) string {
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msg := e.Message
		if e.Reason != "" {
			msg += " (" + e.Reason + ")"
		}
		for _, se := range e.SchemaValidationErrors {
			msg += "; " + se.FieldPath + ": " + se.Reason
		}
		msgs = append(msgs, msg)
	}
	return strings.Join(msgs, "; ")
}

// formDecoder decodes forms like the validator's own decoder, but makes a
// single value of an array property an array. With the default form style,
// `i=1` is the array [1], which the stock decoders (libopenapi-validator
// v0.16.1) leave as the string "1" and then reject.
func formDecoder() content.Decoder {
	stock := content.FormDecoder()
	return content.DecoderFunc(func(in *content.DecodeInput) (any, error) {
		v, err := stock.Decode(in)
		if err != nil || in.Schema == nil || in.Schema.Properties == nil {
			return v, err
		}
		fields, ok := v.(map[string]any)
		if !ok {
			return v, nil
		}
		for name, prop := range in.Schema.Properties.FromOldest() {
			if s, ok := fields[name].(string); ok && slices.Contains(prop.Schema().Type, "array") {
				fields[name] = []any{s}
			}
		}
		return fields, nil
	})
}

// authenticate checks that a request carries the credentials of a security
// scheme. The validator checks API keys on its own, but treats OAuth 2.0 as
// always satisfied; here it needs a bearer token.
func authenticate(_ context.Context, in *config.AuthenticationInput) error {
	r, scheme := in.Request, in.SecurityScheme
	switch strings.ToLower(scheme.Type) {
	case "apikey":
		var v string
		switch scheme.In {
		case "header":
			v = r.Header.Get(scheme.Name)
		case "query":
			v = r.URL.Query().Get(scheme.Name)
		case "cookie":
			if c, err := r.Cookie(scheme.Name); err == nil {
				v = c.Value
			}
		}
		if v == "" {
			return fmt.Errorf("API key %s not found in the %s", scheme.Name, scheme.In)
		}
	case "oauth2", "openidconnect":
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			return errors.New("no bearer token in the Authorization header")
		}
	case "http":
		if !strings.HasPrefix(strings.ToLower(r.Header.Get("Authorization")), strings.ToLower(scheme.Scheme)+" ") {
			return fmt.Errorf("no %s credentials in the Authorization header", scheme.Scheme)
		}
	default:
		return fmt.Errorf("unsupported security scheme type %q", scheme.Type)
	}
	return nil
}

// withBody returns a copy of r that reads body, which the validator
// consumes.
func withBody(r *http.Request, body []byte) *http.Request {
	r2 := r.Clone(r.Context())
	r2.Body = io.NopCloser(bytes.NewReader(body))
	r2.ContentLength = int64(len(body))
	return r2
}
