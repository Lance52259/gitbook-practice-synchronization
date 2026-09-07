package monitor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Lance52259/gitbook-practice-synchronization/internal/config"
	"github.com/Lance52259/gitbook-practice-synchronization/internal/model"
)

var githubOwnerRepo = regexp.MustCompile(`(?i)github\.com[:/](.+?)/(.+?)(?:\.git)?$`)

// RepoContext holds prepared B and C repos.
type RepoContext struct {
	B model.RepoRef
	C model.RepoRef
}

// ToHTTPSURL normalizes owner/name or URL into an https clone URL.
func ToHTTPSURL(repo, token string) string {
	value := strings.TrimSpace(repo)
	value = strings.TrimSuffix(value, ".git")

	ownerName := ""
	if matched, _ := regexp.MatchString(`^[\w.-]+/[\w.-]+$`, value); matched {
		ownerName = value
	} else if m := githubOwnerRepo.FindStringSubmatch(value); len(m) == 3 {
		ownerName = m[1] + "/" + m[2]
	}

	if ownerName != "" {
		if token != "" {
			return fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, ownerName)
		}
		return fmt.Sprintf("https://github.com/%s.git", ownerName)
	}
	return repo
}

// RepoWatcher clones/fetches B and C working trees.
type RepoWatcher struct {
	Settings *config.Settings
}

func (w *RepoWatcher) PrepareRepos(refresh bool) (*RepoContext, error) {
	work := w.Settings.AbsoluteWorkDir()
	if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, err
	}
	bPath := filepath.Join(work, worktreeDirName("b", w.Settings.BRepo))
	cPath := filepath.Join(work, worktreeDirName("c", w.Settings.CRepo))

	fmt.Printf("Preparing B repo %s → %s\n", w.Settings.BRepo, bPath)
	b, err := w.ensureClone(w.Settings.BRepo, w.Settings.BRepoToken, w.Settings.BDefaultBranch, bPath, refresh)
	if err != nil {
		return nil, fmt.Errorf("prepare B (%s): %w", w.Settings.BRepo, err)
	}
	cToken := w.Settings.CRepoToken
	if cToken == "" {
		cToken = w.Settings.BRepoToken
	}
	fmt.Printf("Preparing C repo %s → %s\n", w.Settings.CRepo, cPath)
	c, err := w.ensureClone(w.Settings.CRepo, cToken, w.Settings.CDefaultBranch, cPath, refresh)
	if err != nil {
		return nil, fmt.Errorf("prepare C (%s): %w", w.Settings.CRepo, err)
	}
	return &RepoContext{B: b, C: c}, nil
}

// worktreeDirName builds a stable local folder name from role + owner/repo
// (e.g. b-huaweicloud-terraform-provider-huaweicloud).
func worktreeDirName(role, repo string) string {
	slug := strings.TrimSpace(repo)
	slug = strings.TrimSuffix(slug, ".git")
	if m := githubOwnerRepo.FindStringSubmatch(slug); len(m) == 3 {
		slug = m[1] + "-" + m[2]
	} else {
		slug = strings.ReplaceAll(slug, "/", "-")
		slug = strings.ReplaceAll(slug, ":", "-")
	}
	slug = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, slug)
	if slug == "" {
		slug = "unknown"
	}
	return role + "-" + slug
}

func (w *RepoWatcher) ensureClone(name, token, branch, dest string, refresh bool) (model.RepoRef, error) {
	url := ToHTTPSURL(name, token)
	gitDir := filepath.Join(dest, ".git")

	if st, err := os.Stat(gitDir); err == nil && st.IsDir() {
		if refresh {
			if err := runGit(dest, "fetch", "origin"); err != nil {
				return model.RepoRef{}, err
			}
			if err := runGit(dest, "checkout", branch); err != nil {
				return model.RepoRef{}, err
			}
			_ = runGit(dest, "pull", "origin", branch)
		}
	} else {
		if _, err := os.Stat(dest); err == nil {
			return model.RepoRef{}, fmt.Errorf("work path exists but is not a git repo: %s", dest)
		}
		if err := runGit("", "clone", "--branch", branch, "--depth", "1", url, dest); err != nil {
			return model.RepoRef{}, err
		}
	}

	sha, err := runGitOutput(dest, "rev-parse", "HEAD")
	if err != nil {
		return model.RepoRef{}, err
	}
	return model.RepoRef{
		Name:      name,
		Branch:    branch,
		Token:     token,
		LocalPath: dest,
		CommitSHA: strings.TrimSpace(sha),
	}, nil
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runGitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}
