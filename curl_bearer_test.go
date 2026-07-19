package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
)

func TestDocumentedCurlBearerSendsRealBearerScheme(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl unavailable")
	}
	tokenFile := t.TempDir() + "/token"
	if err := os.WriteFile(tokenFile, []byte("document-test-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	seen := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	script := `{ printf 'header = "Authorization: Bearer '; tr -d '\r\n' < "$TOKEN_FILE"; printf '"\n'; } | curl --silent --show-error --config - "$URL"`
	command := exec.Command("sh", "-c", script)
	command.Env = append(os.Environ(), "TOKEN_FILE="+tokenFile, "URL="+server.URL)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("curl: %v: %s", err, output)
	}
	if got := <-seen; got != "Bearer document-test-token" {
		t.Fatalf("authorization=%q", got)
	}
}
