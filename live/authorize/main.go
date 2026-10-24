// Command authorize gets an OAuth 2.0 refresh token for the live tests and
// writes it to .env as INOREADER_REFRESH_TOKEN, without printing it.
//
// It reads the app from INOREADER_APP_ID and INOREADER_APP_KEY, prints the
// URL of Inoreader's consent page, and waits on localhost for the browser to
// come back with the authorization code. The app must be registered with
// the redirect URI it prints. Run it with `mise run live:authorize`.
package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/uma-31/inoreader-go"
)

func main() {
	port := flag.Int("port", 8765, "localhost port of the redirect URI")
	envFile := flag.String("env-file", ".env", "file to write the refresh token to")
	scope := flag.String("scope", inoreader.ScopeRead, "OAuth 2.0 scope, read or \"read write\"")
	timeout := flag.Duration("timeout", 5*time.Minute, "how long to wait for the authorization")
	flag.Parse()

	if err := run(*port, *envFile, *scope, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, "authorize:", err)
		os.Exit(1)
	}
}

func run(port int, envFile, scope string, timeout time.Duration) error {
	appID, appKey := os.Getenv("INOREADER_APP_ID"), os.Getenv("INOREADER_APP_KEY")
	if appID == "" || appKey == "" {
		return errors.New("set INOREADER_APP_ID and INOREADER_APP_KEY, in .env for instance")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Listen before printing the URL, so a busy port fails early.
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return fmt.Errorf("listening for the redirect: %w", err)
	}

	conf := &oauth2.Config{
		ClientID:     appID,
		ClientSecret: appKey,
		Endpoint:     inoreader.Endpoint,
		RedirectURL:  fmt.Sprintf("http://localhost:%d/callback", port),
		Scopes:       strings.Fields(scope),
	}
	state := rand.Text()

	codes := make(chan string, 1)
	errs := make(chan error, 1)
	srv := &http.Server{
		Handler:           callbackHandler(state, codes, errs),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	fmt.Printf("The app must have this redirect URI: %s\n\n", conf.RedirectURL)
	fmt.Printf("Open this URL and authorize the app:\n\n  %s\n\n", conf.AuthCodeURL(state))

	var code string
	select {
	case code = <-codes:
	case err := <-errs:
		return err
	case <-ctx.Done():
		return errors.New("timed out waiting for the authorization")
	}

	exchangeCtx := context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: 30 * time.Second})
	token, err := conf.Exchange(exchangeCtx, code)
	if err != nil {
		return fmt.Errorf("exchanging the authorization code: %w", err)
	}
	if token.RefreshToken == "" {
		return errors.New("the token response has no refresh token")
	}

	if err := writeEnv(envFile, "INOREADER_REFRESH_TOKEN", token.RefreshToken); err != nil {
		return err
	}
	fmt.Printf("Wrote INOREADER_REFRESH_TOKEN to %s.\n", envFile)
	return nil
}

// callbackHandler receives the browser's redirect, checks state against
// cross-site request forgery, and sends on the authorization code.
func callbackHandler(state string, codes chan<- string, errs chan<- error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			http.Error(w, "state does not match; start again", http.StatusBadRequest)
			return
		}
		if e := q.Get("error"); e != "" {
			http.Error(w, "authorization failed: "+e, http.StatusBadRequest)
			trySend(errs, fmt.Errorf("authorization failed: %s", e))
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "no authorization code", http.StatusBadRequest)
			return
		}

		_, _ = fmt.Fprintln(w, "Authorized. You can close this tab.")
		trySend(codes, code)
	})
	return mux
}

// trySend drops v when a value was sent already, such as on a reload.
func trySend[T any](ch chan<- T, v T) {
	select {
	case ch <- v:
	default:
	}
}

// writeEnv sets key to value in the dotenv file at path, keeping its other
// lines, and leaves the file readable by its owner only.
func writeEnv(path, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".env-*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	// CreateTemp already makes the file 0600.
	if _, err := tmp.WriteString(setEnv(string(data), key, value)); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// setEnv returns the dotenv content with key set to value: the first line
// assigning key is replaced and later ones removed, or a line is appended.
func setEnv(content, key, value string) string {
	line := key + "=" + value
	prefix := key + "="

	var out []string
	replaced := false
	for l := range strings.Lines(content) {
		l = strings.TrimRight(l, "\r\n")
		if strings.HasPrefix(strings.TrimSpace(l), prefix) {
			if !replaced {
				out = append(out, line)
				replaced = true
			}
			continue
		}
		out = append(out, l)
	}
	if !replaced {
		out = append(out, line)
	}
	return strings.Join(out, "\n") + "\n"
}
