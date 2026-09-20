package model

import (
	"fmt"
	"strings"
)

// Practice is one best-practice unit under B repo examples/.
type Practice struct {
	PracticeID string            `json:"practice_id"`
	SourcePath string            `json:"source_path"`
	Title      string            `json:"title"`
	Files      []string          `json:"files"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Slug returns the basename of practice_id.
func (p Practice) Slug() string {
	for i := len(p.PracticeID) - 1; i >= 0; i-- {
		if p.PracticeID[i] == '/' {
			return p.PracticeID[i+1:]
		}
	}
	return p.PracticeID
}

// RelUnderService returns the path under the service directory.
// examples/dms/kafka/instance-configuration → kafka/instance-configuration
// examples/dns/zone → zone
func (p Practice) RelUnderService() string {
	parts := splitPath(p.PracticeID)
	if len(parts) >= 3 && parts[0] == "examples" {
		return strings.Join(parts[2:], "/")
	}
	if len(parts) >= 2 && parts[0] != "examples" {
		return strings.Join(parts[1:], "/")
	}
	return p.Slug()
}

// Service returns the service segment for paths like examples/ecs/basic → ecs.
func (p Practice) Service() string {
	parts := splitPath(p.PracticeID)
	if len(parts) >= 3 && parts[0] == "examples" {
		return parts[1]
	}
	if len(parts) >= 2 && parts[0] != "examples" {
		return parts[0]
	}
	return ""
}

// SimpleTitle derives a short English scenario title from the practice directory name.
// Example: rds/basic_instance → "basic instance".
// Redundant service-name prefixes (rds_, rds-, "rds ", sfs-turbo_, …) are stripped.
func (p Practice) SimpleTitle() string {
	return SimplePracticeTitle(p.Service(), p.Slug())
}

// CommitTitle returns the fixed commit/PR title format:
//
//	docs({service}): support new best practice for {simple title}
func (p Practice) CommitTitle() string {
	service := p.Service()
	if service == "" {
		service = "unknown"
	}
	return fmt.Sprintf("docs(%s): support new best practice for %s", service, p.SimpleTitle())
}

// SimplePracticeTitle builds a human-readable title from service + slug.
func SimplePracticeTitle(service, slug string) string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "unknown"
	}

	trimmed := stripServicePrefix(slug, service)
	title := strings.ReplaceAll(trimmed, "-", " ")
	title = strings.ReplaceAll(title, "_", " ")
	title = collapseSpaces(title)
	title = stripServiceWordPrefix(title, service)
	title = collapseSpaces(title)
	if title == "" {
		title = strings.ReplaceAll(strings.ReplaceAll(slug, "-", " "), "_", " ")
		title = collapseSpaces(title)
	}
	return title
}

func stripServicePrefix(slug, service string) string {
	if service == "" {
		return slug
	}
	variants := serviceNameVariants(service)
	lower := strings.ToLower(slug)
	for _, v := range variants {
		for _, sep := range []string{"_", "-"} {
			prefix := strings.ToLower(v + sep)
			if strings.HasPrefix(lower, prefix) && len(slug) > len(prefix) {
				return slug[len(prefix):]
			}
		}
	}
	return slug
}

func stripServiceWordPrefix(title, service string) string {
	if service == "" || title == "" {
		return title
	}
	for _, v := range serviceNameVariants(service) {
		words := collapseSpaces(strings.ReplaceAll(strings.ReplaceAll(v, "-", " "), "_", " "))
		if words == "" {
			continue
		}
		prefix := strings.ToLower(words) + " "
		if strings.HasPrefix(strings.ToLower(title), prefix) {
			return strings.TrimSpace(title[len(prefix):])
		}
	}
	return title
}

func serviceNameVariants(service string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	add(service)
	add(strings.ReplaceAll(service, "-", "_"))
	add(strings.ReplaceAll(service, "_", "-"))
	add(strings.ReplaceAll(strings.ReplaceAll(service, "-", ""), "_", ""))
	return out
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func splitPath(p string) []string {
	var out []string
	start := 0
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			if i > start {
				out = append(out, p[start:i])
			}
			start = i + 1
		}
	}
	if start < len(p) {
		out = append(out, p[start:])
	}
	return out
}

// DocFileChange is one generated file write.
type DocFileChange struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Action  string `json:"action"`
}

// GenerateResult is AI output for one practice.
type GenerateResult struct {
	PracticeID  string          `json:"practice_id"`
	Files       []DocFileChange `json:"files"`
	Summary     string          `json:"summary"`
	RawResponse string          `json:"-"`
}

// DetectionResult is the outcome of B vs C comparison.
type DetectionResult struct {
	NewPractices  []Practice `json:"new_practices"`
	SyncedIDs     []string   `json:"synced_ids"`
	SkippedOpenPR []string   `json:"skipped_open_pr,omitempty"`
	BCommit       string     `json:"b_commit,omitempty"`
	CCommit       string     `json:"c_commit,omitempty"`
}

// PipelineResult summarizes a full run.
type PipelineResult struct {
	Detected  DetectionResult  `json:"detected"`
	Generated []GenerateResult `json:"generated"`
	PRURLs    []string         `json:"pr_urls"`
	Skipped   []string         `json:"skipped"`
	Errors    []string         `json:"errors"`
	DryRun    bool             `json:"dry_run"`
}

// RepoRef describes a prepared working tree.
type RepoRef struct {
	Name      string
	Branch    string
	Token     string
	LocalPath string
	CommitSHA string
}
