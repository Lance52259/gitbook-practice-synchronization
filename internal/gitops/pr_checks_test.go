package gitops_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chnsz/gitbook-practice-synchronization/internal/config"
	"github.com/chnsz/gitbook-practice-synchronization/internal/gitops"
)

func TestGetCommitChecksStatus_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/commits/abc/check-runs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_count": 1,
			"check_runs": []map[string]any{
				{"name": "test", "status": "completed", "conclusion": "success"},
			},
		})
	})
	mux.HandleFunc("/repos/o/r/commits/abc/status", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"state":       "success",
			"total_count": 0,
			"statuses":    []any{},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prm, err := gitops.NewPRManager(&config.Settings{CRepo: "o/r", CRepoToken: "t"})
	if err != nil {
		t.Fatal(err)
	}
	prm.APIBase = srv.URL
	prm.Client = srv.Client()

	st, err := prm.GetCommitChecksStatus("abc")
	if err != nil {
		t.Fatal(err)
	}
	if st.Outcome != gitops.ChecksSuccess {
		t.Fatalf("got %+v", st)
	}
}

func TestGetCommitChecksStatus_PendingEmpty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/commits/abc/check-runs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"total_count": 0, "check_runs": []any{}})
	})
	mux.HandleFunc("/repos/o/r/commits/abc/status", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"state": "pending", "total_count": 0, "statuses": []any{}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prm, _ := gitops.NewPRManager(&config.Settings{CRepo: "o/r", CRepoToken: "t"})
	prm.APIBase = srv.URL
	prm.Client = srv.Client()
	st, err := prm.GetCommitChecksStatus("abc")
	if err != nil {
		t.Fatal(err)
	}
	if st.Outcome != gitops.ChecksPending {
		t.Fatalf("got %+v", st)
	}
}

func TestWaitPRChecks_TimeoutPending(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/pulls/7", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"head": map[string]any{"sha": "deadbeef"}})
	})
	mux.HandleFunc("/repos/o/r/commits/deadbeef/check-runs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_count": 1,
			"check_runs": []map[string]any{
				{"name": "ci", "status": "in_progress", "conclusion": nil},
			},
		})
	})
	mux.HandleFunc("/repos/o/r/commits/deadbeef/status", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"state": "pending", "total_count": 0, "statuses": []any{}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prm, _ := gitops.NewPRManager(&config.Settings{CRepo: "o/r", CRepoToken: "t"})
	prm.APIBase = srv.URL
	prm.Client = srv.Client()

	st, ok, err := prm.WaitPRChecks(7, 30*time.Millisecond, 5*time.Millisecond, func(d time.Duration) {
		time.Sleep(d)
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected timeout not ok")
	}
	if st.Outcome != gitops.ChecksPending {
		t.Fatalf("got %+v", st)
	}
}

func TestWaitPRChecks_SuccessThenStop(t *testing.T) {
	n := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/pulls/3", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"head": map[string]any{"sha": "cafe"}})
	})
	mux.HandleFunc("/repos/o/r/commits/cafe/check-runs", func(w http.ResponseWriter, r *http.Request) {
		n++
		if n < 2 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total_count": 1,
				"check_runs":  []map[string]any{{"name": "ci", "status": "queued", "conclusion": nil}},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_count": 1,
			"check_runs":  []map[string]any{{"name": "ci", "status": "completed", "conclusion": "success"}},
		})
	})
	mux.HandleFunc("/repos/o/r/commits/cafe/status", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"state": "success", "total_count": 0, "statuses": []any{}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prm, _ := gitops.NewPRManager(&config.Settings{CRepo: "o/r", CRepoToken: "t"})
	prm.APIBase = srv.URL
	prm.Client = srv.Client()

	st, ok, err := prm.WaitPRChecks(3, time.Second, time.Millisecond, func(d time.Duration) { time.Sleep(d) })
	if err != nil {
		t.Fatal(err)
	}
	if !ok || st.Outcome != gitops.ChecksSuccess {
		t.Fatalf("ok=%v st=%+v", ok, st)
	}
}
