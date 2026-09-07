package gitops_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Lance52259/gitbook-practice-synchronization/internal/config"
	"github.com/Lance52259/gitbook-practice-synchronization/internal/gitops"
	"github.com/Lance52259/gitbook-practice-synchronization/internal/model"
)

func TestPracticeBranch(t *testing.T) {
	got := gitops.PracticeBranch("examples/antiddos/basic")
	want := "gitbook-practice-synchronization/examples-antiddos-basic"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestServiceFromPRTitle(t *testing.T) {
	if got := gitops.ServiceFromPRTitle("docs(dcs): support new best practice for redis account"); got != "dcs" {
		t.Fatalf("got %q", got)
	}
	if got := gitops.ServiceFromPRTitle("docs(anti-ddos): support new best practice for basic"); got != "anti-ddos" {
		t.Fatalf("got %q", got)
	}
	if got := gitops.ServiceFromPRTitle("chore: unrelated"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestFilterOpenPRs_ServiceAndOnePerScan(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/Lance52259/hcbp-demo/pulls", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != "open" {
			t.Fatalf("state=%s", r.URL.Query().Get("state"))
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"number":   11,
				"html_url": "https://github.com/Lance52259/hcbp-demo/pull/11",
				"title":    "docs(dcs): support new best practice for redis account",
				"head":     map[string]any{"ref": "gitbook-practice-synchronization/examples-dcs-redis-account"},
			},
			{
				"number":   99,
				"html_url": "https://github.com/Lance52259/hcbp-demo/pull/99",
				"title":    "docs: missing service",
				"head":     map[string]any{"ref": "feature/other"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prm, err := gitops.NewPRManager(&config.Settings{
		CRepo:      "Lance52259/hcbp-demo",
		CRepoToken: "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	prm.APIBase = srv.URL
	prm.Client = srv.Client()

	practices := []model.Practice{
		{PracticeID: "examples/dcs/redis-all-sessions-kill"},
		{PracticeID: "examples/dcs/redis-account"},
		{PracticeID: "examples/vpc/basic"},
		{PracticeID: "examples/vpc/peering"},
		{PracticeID: "examples/ecs/basic"},
	}
	serviceOf := func(p model.Practice) string { return p.Service() }
	keep, skipped, err := prm.FilterOpenPRs(practices, serviceOf)
	if err != nil {
		t.Fatal(err)
	}
	if len(keep) != 2 {
		t.Fatalf("keep=%+v skipped=%v", keep, skipped)
	}
	if keep[0].PracticeID != "examples/vpc/basic" || keep[1].PracticeID != "examples/ecs/basic" {
		t.Fatalf("keep=%+v", keep)
	}
	joined := strings.Join(skipped, "\n")
	for _, want := range []string{
		"examples/dcs/redis-all-sessions-kill",
		`service "dcs"`,
		"examples/vpc/peering",
		"already queued in this scan",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("skipped missing %q:\n%s", want, joined)
		}
	}
}
