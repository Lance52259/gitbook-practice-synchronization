package gitops

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/Lance52259/doc-draft/internal/model"
)

var docsServiceTitleRe = regexp.MustCompile(`(?i)^docs\(([^)]+)\):\s*`)

// PracticeBranch returns the head branch for a practice_id
// (gitbook-practice-synchronization convention: doc-craft/<practice-id-with-dashes>).
// Example: examples/antiddos/basic → doc-craft/examples-antiddos-basic
func PracticeBranch(practiceID string) string {
	slug := strings.ReplaceAll(practiceID, "/", "-")
	name := "doc-craft/" + slug
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

// OpenDocCraftPR is an open PR whose head branch uses the doc-craft/ prefix.
type OpenDocCraftPR struct {
	Number  int
	URL     string
	Title   string
	Head    string
	Service string
}

// ListOpenDocCraftPRs lists open PRs whose head ref starts with doc-craft/.
func (m *PRManager) ListOpenDocCraftPRs() ([]OpenDocCraftPR, error) {
	headers, err := m.headers()
	if err != nil {
		return nil, err
	}
	var out []OpenDocCraftPR
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
			if !strings.HasPrefix(head, "doc-craft/") {
				continue
			}
			svc := ServiceFromPRTitle(it.Title)
			out = append(out, OpenDocCraftPR{
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
// gitbook-practice-synchronization PR (head under doc-craft/),
// and keeps at most one practice per service in this scan (first wins).
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

	open, err := m.ListOpenDocCraftPRs()
	if err != nil {
		return nil, nil, err
	}

	// service → representative open PR (first seen)
	blocked := map[string]OpenDocCraftPR{}
	for _, pr := range open {
		svc := strings.ToLower(strings.TrimSpace(pr.Service))
		if svc == "" {
			// Title missing docs(service): — still block exact practice branch match below via head.
			continue
		}
		if _, ok := blocked[svc]; !ok {
			blocked[svc] = pr
		}
	}

	// Exact head match when title lacked a parseable service.
	openByHead := map[string]OpenDocCraftPR{}
	for _, pr := range open {
		openByHead[pr.Head] = pr
	}

	seenService := map[string]string{} // service → practice_id kept in this scan
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
