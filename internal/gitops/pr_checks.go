package gitops

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChecksOutcome is the rollup of PR head commit checks / statuses.
type ChecksOutcome string

const (
	ChecksPending ChecksOutcome = "pending"
	ChecksSuccess ChecksOutcome = "success"
	ChecksFailure ChecksOutcome = "failure"
)

// ChecksStatus summarizes CI for a PR head.
type ChecksStatus struct {
	Outcome ChecksOutcome
	SHA     string
	Summary string
	Details []string
}

// GetPRHeadSHA returns the current head SHA for a pull request.
func (m *PRManager) GetPRHeadSHA(prNumber int) (string, error) {
	headers, err := m.headers()
	if err != nil {
		return "", err
	}
	url := m.api(fmt.Sprintf("/repos/%s/%s/pulls/%d", m.Owner, m.Repo, prNumber))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header = headers
	resp, err := m.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("get PR #%d HTTP %d: %s", prNumber, resp.StatusCode, string(body))
	}
	var item struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(body, &item); err != nil {
		return "", err
	}
	if strings.TrimSpace(item.Head.SHA) == "" {
		return "", fmt.Errorf("PR #%d has empty head sha", prNumber)
	}
	return item.Head.SHA, nil
}

// GetCommitChecksStatus rolls up check-runs and combined commit status for a SHA.
func (m *PRManager) GetCommitChecksStatus(sha string) (ChecksStatus, error) {
	out := ChecksStatus{SHA: sha, Outcome: ChecksPending}
	headers, err := m.headers()
	if err != nil {
		return out, err
	}

	checkURL := m.api(fmt.Sprintf("/repos/%s/%s/commits/%s/check-runs?per_page=100", m.Owner, m.Repo, sha))
	req, err := http.NewRequest(http.MethodGet, checkURL, nil)
	if err != nil {
		return out, err
	}
	req.Header = headers
	resp, err := m.Client.Do(req)
	if err != nil {
		return out, err
	}
	checkBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return out, fmt.Errorf("list check-runs HTTP %d: %s", resp.StatusCode, string(checkBody))
	}
	var checks struct {
		TotalCount int `json:"total_count"`
		CheckRuns  []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"check_runs"`
	}
	if err := json.Unmarshal(checkBody, &checks); err != nil {
		return out, err
	}

	statusURL := m.api(fmt.Sprintf("/repos/%s/%s/commits/%s/status", m.Owner, m.Repo, sha))
	req2, err := http.NewRequest(http.MethodGet, statusURL, nil)
	if err != nil {
		return out, err
	}
	req2.Header = headers
	resp2, err := m.Client.Do(req2)
	if err != nil {
		return out, err
	}
	statusBody, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode >= 300 {
		return out, fmt.Errorf("combined status HTTP %d: %s", resp2.StatusCode, string(statusBody))
	}
	var combined struct {
		State      string `json:"state"`
		TotalCount int    `json:"total_count"`
		Statuses   []struct {
			Context string `json:"context"`
			State   string `json:"state"`
		} `json:"statuses"`
	}
	if err := json.Unmarshal(statusBody, &combined); err != nil {
		return out, err
	}

	var pending, failing, passing int
	for _, cr := range checks.CheckRuns {
		name := cr.Name
		if name == "" {
			name = "(check)"
		}
		st := strings.ToLower(strings.TrimSpace(cr.Status))
		conc := strings.ToLower(strings.TrimSpace(cr.Conclusion))
		if st != "completed" {
			pending++
			out.Details = append(out.Details, fmt.Sprintf("check %q status=%s", name, st))
			continue
		}
		switch conc {
		case "success", "skipped", "neutral":
			passing++
			out.Details = append(out.Details, fmt.Sprintf("check %q conclusion=%s", name, conc))
		case "failure", "cancelled", "timed_out", "action_required", "startup_failure":
			failing++
			out.Details = append(out.Details, fmt.Sprintf("check %q conclusion=%s", name, conc))
		default:
			// empty or unknown conclusion while completed — treat as pending/unknown
			pending++
			out.Details = append(out.Details, fmt.Sprintf("check %q conclusion=%q", name, conc))
		}
	}
	for _, st := range combined.Statuses {
		ctx := st.Context
		if ctx == "" {
			ctx = "(status)"
		}
		state := strings.ToLower(strings.TrimSpace(st.State))
		out.Details = append(out.Details, fmt.Sprintf("status %q state=%s", ctx, state))
		switch state {
		case "success":
			passing++
		case "failure", "error":
			failing++
		default:
			pending++
		}
	}

	totalObserved := len(checks.CheckRuns) + len(combined.Statuses)
	if totalObserved == 0 {
		out.Outcome = ChecksPending
		out.Summary = "no check-runs or commit statuses reported yet"
		return out, nil
	}
	if failing > 0 || strings.EqualFold(combined.State, "failure") || strings.EqualFold(combined.State, "error") {
		out.Outcome = ChecksFailure
		out.Summary = fmt.Sprintf("checks failed (failing=%d pending=%d passing=%d combined=%s)", failing, pending, passing, combined.State)
		return out, nil
	}
	if pending > 0 {
		out.Outcome = ChecksPending
		out.Summary = fmt.Sprintf("checks pending (pending=%d passing=%d combined=%s)", pending, passing, combined.State)
		return out, nil
	}
	// Combined "pending" with zero status contexts is common when only Check Runs exist — ignore.
	if strings.EqualFold(combined.State, "pending") && combined.TotalCount > 0 {
		out.Outcome = ChecksPending
		out.Summary = fmt.Sprintf("commit statuses pending (passing=%d combined=%s)", passing, combined.State)
		return out, nil
	}
	out.Outcome = ChecksSuccess
	out.Summary = fmt.Sprintf("checks passed (passing=%d combined=%s)", passing, combined.State)
	return out, nil
}

// GetPRChecksStatus loads head SHA then rolls up checks.
func (m *PRManager) GetPRChecksStatus(prNumber int) (ChecksStatus, error) {
	sha, err := m.GetPRHeadSHA(prNumber)
	if err != nil {
		return ChecksStatus{Outcome: ChecksPending}, err
	}
	return m.GetCommitChecksStatus(sha)
}

// WaitPRChecks polls until success/failure or timeout.
// On timeout the returned status is the last observed pending snapshot (Outcome stays pending).
// sleep may be overridden in tests; nil uses time.Sleep.
func (m *PRManager) WaitPRChecks(prNumber int, timeout, interval time.Duration, sleep func(time.Duration)) (ChecksStatus, bool, error) {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if sleep == nil {
		sleep = time.Sleep
	}
	deadline := time.Now().Add(timeout)
	var last ChecksStatus
	for {
		st, err := m.GetPRChecksStatus(prNumber)
		if err != nil {
			return st, false, err
		}
		last = st
		fmt.Printf("PR #%d checks: %s — %s\n", prNumber, st.Outcome, st.Summary)
		switch st.Outcome {
		case ChecksSuccess:
			return st, true, nil
		case ChecksFailure:
			return st, false, nil
		}
		if !time.Now().Before(deadline) {
			return last, false, nil
		}
		remaining := time.Until(deadline)
		wait := interval
		if wait > remaining {
			wait = remaining
		}
		if wait > 0 {
			sleep(wait)
		}
	}
}
