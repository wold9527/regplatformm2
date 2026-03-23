package tempmail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMailTMProviderFetchVerificationCode(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"hydra:member": []map[string]interface{}{
				{
					"id":      "msg-1",
					"subject": "Google verification code: ABC123",
					"date":    "2026-03-18T14:05:00Z",
				},
			},
		})
	})
	mux.HandleFunc("/messages/msg-1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"text": "Google verification code: ABC123",
		})
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	provider := &mailTMProvider{baseURL: server.URL, name: "mailtm"}
	code, err := provider.FetchVerificationCode(context.Background(), "user@example.com", map[string]string{
		"token": "mailtm-token",
	}, 1, 0)
	if err != nil {
		t.Fatalf("FetchVerificationCode() error = %v", err)
	}
	if code != "ABC123" {
		t.Fatalf("FetchVerificationCode() = %q, want %q", code, "ABC123")
	}
}

func TestFetchVerificationCodeByProviderMailfree(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/emails", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"id":                "mail-1",
				"verification_code": "ZX91Q2",
			},
		})
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	code, err := FetchVerificationCodeByProvider(context.Background(), "mailfree", map[string]string{}, "user@example.com", map[string]string{
		"base_url":    server.URL,
		"admin_token": "mailfree-token",
	})
	if err != nil {
		t.Fatalf("FetchVerificationCodeByProvider() error = %v", err)
	}
	if code != "ZX91Q2" {
		t.Fatalf("FetchVerificationCodeByProvider() = %q, want %q", code, "ZX91Q2")
	}
}
