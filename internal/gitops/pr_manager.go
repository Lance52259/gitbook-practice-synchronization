package gitops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/chnsz/gitbook-practice-synchronization/internal/config"
)

var ownerRepoRe = regexp.MustCompile(`(?i)github\.com[:/](.+?)/(.+?)(?:\.git)?$`)

// PullRequest is a GitHub PR reference.
type PullRequest struct {
	Number int
	URL    string
	Title  string
}

// ParseOwnerRepo extracts owner/name from a repo string.
func ParseOwnerRepo(repo string) (string, string, error) {
	value := strings.TrimSpace(repo)
	if matched, _ := regexp.MatchString(`^[\w.-]+/[\w.-]+$`, value); matched {
		parts := strings.SplitN(value, "/", 2)
		return parts[0], parts[1], nil
	}
	m := ownerRepoRe.FindStringSubmatch(value)
	if len(m) != 3 {
		return "", "", fmt.Errorf("cannot parse owner/repo from: %s", repo)
	}
	return m[1], m[2], nil
}

// PRManager creates/updates PRs via GitHub API.
type PRManager struct {
	Settings *config.Settings
	Owner    string
	Repo     string
	Client   *http.Client
	// APIBase defaults to https://api.github.com (override in tests).
	APIBase string
}

func NewPRManager(settings *config.Settings) (*PRManager, error) {
	owner, repo, err := ParseOwnerRepo(settings.CRepo)
	if err != nil {
		return nil, err
	}
	return &PRManager{
		Settings: settings,
		Owner:    owner,
		Repo:     repo,
		Client:   &http.Client{Timeout: 60 * time.Second},
		APIBase:  "https://api.github.com",
	}, nil
}

func (m *PRManager) api(pathQuery string) string {
	base := m.APIBase
	if base == "" {
		base = "https://api.github.com"
	}
	return strings.TrimRight(base, "/") + pathQuery
}

func (m *PRManager) headers() (http.Header, error) {
	if m.Settings.CRepoToken == "" {
		return nil, fmt.Errorf("C_REPO_TOKEN is required to manage PRs")
	}
	h := make(http.Header)
	h.Set("Authorization", "Bearer "+m.Settings.CRepoToken)
	h.Set("Accept", "application/vnd.github+json")
	h.Set("X-GitHub-Api-Version", "2022-11-28")
	return h, nil
}

func (m *PRManager) FindOpenPR(headBranch string) (*PullRequest, error) {
	headers, err := m.headers()
	if err != nil {
		return nil, err
	}
	url := m.api(fmt.Sprintf("/repos/%s/%s/pulls?state=open&head=%s:%s", m.Owner, m.Repo, m.Owner, headBranch))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header = headers
	resp, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list PRs HTTP %d: %s", resp.StatusCode, string(body))
	}
	var items []struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
		Title   string `json:"title"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &PullRequest{Number: items[0].Number, URL: items[0].HTMLURL, Title: items[0].Title}, nil
}

func (m *PRManager) CreatePR(title, body, head, base string, dryRun bool) (*PullRequest, error) {
	// Dry-run without token: do not call GitHub API.
	if dryRun && strings.TrimSpace(m.Settings.CRepoToken) == "" {
		return &PullRequest{Number: 0, URL: "dry-run://pr", Title: title}, nil
	}

	existing, err := m.FindOpenPR(head)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if m.Settings.PRUpdateMode == "skip" || dryRun {
			return existing, nil
		}
		headers, err := m.headers()
		if err != nil {
			return nil, err
		}
		payload, _ := json.Marshal(map[string]string{"title": title, "body": body})
		url := m.api(fmt.Sprintf("/repos/%s/%s/pulls/%d", m.Owner, m.Repo, existing.Number))
		req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header = headers
		req.Header.Set("Content-Type", "application/json")
		resp, err := m.Client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("update PR HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		var item struct {
			Number  int    `json:"number"`
			HTMLURL string `json:"html_url"`
			Title   string `json:"title"`
		}
		_ = json.Unmarshal(respBody, &item)
		return &PullRequest{Number: item.Number, URL: item.HTMLURL, Title: item.Title}, nil
	}

	if dryRun {
		return &PullRequest{Number: 0, URL: "dry-run://pr", Title: title}, nil
	}

	headers, err := m.headers()
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
		"head":  head,
		"base":  base,
	})
	url := m.api(fmt.Sprintf("/repos/%s/%s/pulls", m.Owner, m.Repo))
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header = headers
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("create PR HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var item struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
		Title   string `json:"title"`
	}
	if err := json.Unmarshal(respBody, &item); err != nil {
		return nil, err
	}
	return &PullRequest{Number: item.Number, URL: item.HTMLURL, Title: item.Title}, nil
}
