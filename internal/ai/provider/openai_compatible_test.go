package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Lance52259/gitbook-practice-synchronization/internal/ai/provider"
)

func TestDeepSeekOpenAICompatible(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "deepseek-chat" {
			t.Fatalf("model=%v", body["model"])
		}
		if int(body["max_tokens"].(float64)) != 300000 {
			t.Fatalf("max_tokens=%v", body["max_tokens"])
		}
		rf, _ := body["response_format"].(map[string]any)
		if rf["type"] != "json_object" {
			t.Fatalf("response_format=%v", body["response_format"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "deepseek-chat",
			"choices": []map[string]any{
				{"message": map[string]any{"content": `{"ok": true}`}},
			},
		})
	}))
	defer srv.Close()

	p := provider.NewDeepSeek("sk-test", srv.URL, "deepseek-chat", 30, 0, 300000)
	res, err := p.Complete(context.Background(), []provider.ChatMessage{{Role: "user", Content: "hi"}}, 0.2, true)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(res.Content), &parsed); err != nil || parsed["ok"] != true {
		t.Fatalf("%v %v", res.Content, err)
	}
}
