package gitops

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/Lance52259/gitbook-practice-synchronization/internal/model"
)

var docsServiceTitleRe = regexp.MustCompile(`(?i)^docs\(([^)]+)\):\s*`)

// BranchPrefix is the head-branch prefix for PRs opened by this tool.
const BranchPrefix = "gitbook-practice-synchronization/"

// PracticeBranch returns the head branch for a practice_id.
// Example: examples/antiddos/basic → gitbook-practice-synchronization/examples-antiddos-basic
func PracticeBranch(practiceID string) string {
	slug := strings.ReplaceAll(practiceID, "/", "-")
	name := BranchPrefix + slug
	if len(name) > 200 {
		return name[:200]
	}
	return name
}

// ServiceFromPRTitle extracts the C-repo service from a PR title.
// Example: docs(dcs): support new best practice for redis account → dcs
func ServiceFromPRTitle(title string) string {
	m := docsServiceTitleRe.FindStringSubmatch(strings.TrimSpace(title))
	if m == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(m[1]))
}

// OpenSyncPR is an open PR whose head branch was created by this tool.
type OpenSyncPR struct {
	Number  int
	URL     string
	Title   string
	Head    string
	Service string
}

// ListOpenSyncPRs lists open PRs whose head ref starts with BranchPrefix.
func (m *PRManager) ListOpenSyncPRs() ([]OpenSyncPR, error) {
	headers, err := m.headers()
	if err != nil {
		return nil, err
	}
	var out []OpenSyncPR
	page := 1
	for {
		url := m.api(fmt.Sprintf("/repos/%s/%s/pulls?state=open&per_page=100&page=%d", m.Owner, m.Repo, page))
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header = headers
		resp, err := m.Client.Do(req)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("list open PRs HTTP %d: %s", resp.StatusCode, string(body))
		}
		var items []struct {
			Number  int    `json:"number"`
			HTMLURL string `json:"html_url"`
			Title   string `json:"title"`
			Head    struct {
				Ref string `json:"ref"`
			} `json:"head"`
		}
		if err := json.Unmarshal(body, &items); err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		for _, it := range items {
			head := strings.TrimSpace(it.Head.Ref)
			if !strings.HasPrefix(head, BranchPrefix) {
				continue
			}
			svc := ServiceFromPRTitle(it.Title)
			out = append(out, OpenSyncPR{
				Number:  it.Number,
				URL:     it.HTMLURL,
				Title:   it.Title,
				Head:    head,
				Service: svc,
			})
		}
		if len(items) < 100 {
			break
		}
		page++
	}
	return out, nil
}

// FilterOpenPRs skips practices whose C-repo service already has an open
// gitbook-practice-synchronization PR, and keeps at most one practice per
// service in this scan (first wins).
//
// serviceOf must return the canonical C docs service directory (after aliases), e.g. anti-ddos.
func (m *PRManager) FilterOpenPRs(practices []model.Practice, serviceOf func(model.Practice) string) (keep []model.Practice, skipped []string, err error) {
	if serviceOf == nil {
		serviceOf = func(p model.Practice) string {
			s := strings.ToLower(strings.TrimSpace(p.Service()))
			if s == "" {
				return "unknown"
			}
			return s
		}
	}

	open, err := m.ListOpenSyncPRs()
	if err != nil {
		return nil, nil, err
	}

	blocked := map[string]OpenSyncPR{}
	for _, pr := range open {
		svc := strings.ToLower(strings.TrimSpace(pr.Service))
		if svc == "" {
			continue
		}
		if _, ok := blocked[svc]; !ok {
			blocked[svc] = pr
		}
	}

	openByHead := map[string]OpenSyncPR{}
	for _, pr := range open {
		openByHead[pr.Head] = pr
	}

	seenService := map[string]string{}
	for _, p := range practices {
		svc := strings.ToLower(strings.TrimSpace(serviceOf(p)))
		if svc == "" {
			svc = "unknown"
		}
		branch := PracticeBranch(p.PracticeID)

		if pr, ok := openByHead[branch]; ok {
			msg := fmt.Sprintf("%s: skip, open PR #%d %s (head %s)", p.PracticeID, pr.Number, pr.URL, branch)
			skipped = append(skipped, msg)
			continue
		}
		if pr, ok := blocked[svc]; ok {
			msg := fmt.Sprintf("%s: skip, service %q already has open PR #%d %s — wait until merge, then next scan",
				p.PracticeID, svc, pr.Number, pr.URL)
			skipped = append(skipped, msg)
			continue
		}
		if prev, ok := seenService[svc]; ok {
			msg := fmt.Sprintf("%s: skip, service %q already queued in this scan (%s) — one open PR per service",
				p.PracticeID, svc, prev)
			skipped = append(skipped, msg)
			continue
		}

		seenService[svc] = p.PracticeID
		keep = append(keep, p)
	}
	return keep, skipped, nil
}
