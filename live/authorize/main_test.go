package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSetEnv(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"empty file", "", "K=new\n"},
		{"appended", "A=1\nB=2\n", "A=1\nB=2\nK=new\n"},
		{"no trailing newline", "A=1", "A=1\nK=new\n"},
		{"replaced in place", "A=1\nK=old\nB=2\n", "A=1\nK=new\nB=2\n"},
		{"duplicates removed", "K=old\nA=1\nK=older\n", "K=new\nA=1\n"},
		{"similar key kept", "KK=1\n", "KK=1\nK=new\n"},
		{"CRLF", "A=1\r\nK=old\r\n", "A=1\nK=new\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := setEnv(tt.content, "K", "new"); got != tt.want {
				t.Errorf("setEnv() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("INOREADER_APP_ID=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeEnv(path, "INOREADER_REFRESH_TOKEN", "secret"); err != nil {
		t.Fatalf("writeEnv: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "INOREADER_APP_ID=1\nINOREADER_REFRESH_TOKEN=secret\n"; string(data) != want {
		t.Errorf("content = %q, want %q", data, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %v, want 0600", perm)
	}
}

func TestCallbackHandler(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		status   int
		wantCode string
		wantErr  bool
	}{
		{"authorized", "?state=s&code=c", http.StatusOK, "c", false},
		{"wrong state", "?state=x&code=c", http.StatusBadRequest, "", false},
		{"no state", "?code=c", http.StatusBadRequest, "", false},
		{"denied", "?state=s&error=access_denied", http.StatusBadRequest, "", true},
		{"no code", "?state=s", http.StatusBadRequest, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codes := make(chan string, 1)
			errs := make(chan error, 1)
			rec := httptest.NewRecorder()
			callbackHandler("s", codes, errs).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback"+tt.query, nil))

			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}
			select {
			case code := <-codes:
				if code != tt.wantCode {
					t.Errorf("code = %q, want %q", code, tt.wantCode)
				}
			default:
				if tt.wantCode != "" {
					t.Error("no code sent")
				}
			}
			if gotErr := len(errs) > 0; gotErr != tt.wantErr {
				t.Errorf("error sent = %v, want %v", gotErr, tt.wantErr)
			}
		})
	}
}
