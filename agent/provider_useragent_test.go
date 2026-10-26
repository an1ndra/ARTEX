package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Autumn-27/norma/transcript"
)

// OpenCode Go asks every client to identify itself with its own User-Agent rather
// than a generic HTTP-library name, and mandates a session header for routing and
// prompt caching. Both must reach the wire from the shared LLM transport.
func TestLLMTransportSendsUserAgentAndSessionHeader(t *testing.T) {
	var gotUA, gotSession string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.UserAgent()
		gotSession = r.Header.Get("x-opencode-session")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := quotaAwareHTTPClient("", "x-opencode-session")
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	ctx := transcript.WithSessionID(context.Background(), "conv-abc123")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = resp.Body.Close()

	want := LLMUserAgent
	if want == "" {
		t.Fatalf("LLMUserAgent must default to something identifying the client")
	}
	if gotUA != want {
		t.Fatalf("User-Agent = %q, want %q", gotUA, want)
	}
	if gotUA == "Go-http-client/1.1" {
		t.Fatalf("generic Go user agent reached the wire")
	}
	if gotSession != "conv-abc123" {
		t.Fatalf("x-opencode-session = %q, want conv-abc123", gotSession)
	}
}

// An explicit User-Agent must not be clobbered by the transport default.
func TestLLMTransportKeepsExplicitUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.UserAgent()
	}))
	defer srv.Close()

	client, err := quotaAwareHTTPClient("", "")
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("User-Agent", "custom/9.9")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = resp.Body.Close()
	if gotUA != "custom/9.9" {
		t.Fatalf("User-Agent = %q, want custom/9.9 (explicit agent overwritten)", gotUA)
	}
}
