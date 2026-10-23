// Command mock serves the Inoreader API from its OpenAPI description, for
// manual checks. Run it with `mise run mock`; the E2E tests start their own.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/uma-31/inoreader-go/e2e/mock"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:4010", "address to listen on")
	spec := flag.String("spec", "api/openapi.yaml", "OpenAPI description to serve")
	flag.Parse()

	if err := run(*addr, *spec); err != nil {
		fmt.Fprintln(os.Stderr, "mock:", err)
		os.Exit(1)
	}
}

func run(addr, spec string) error {
	data, err := os.ReadFile(spec)
	if err != nil {
		return err
	}
	srv, err := mock.New(data, func(v string) { log.Print("violation: ", v) })
	if err != nil {
		return err
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.RequestURI())
		srv.ServeHTTP(w, r)
	})
	log.Printf("serving %s at http://%s", spec, addr)
	err = (&http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}).ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
