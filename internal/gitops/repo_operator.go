package gitops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Lance52259/gitbook-practice-synchronization/internal/config"
	"github.com/Lance52259/gitbook-practice-synchronization/internal/model"
	"github.com/Lance52259/gitbook-practice-synchronization/internal/monitor"
)

// RepoOperator applies generated files and pushes a branch.
type RepoOperator struct {
	Settings *config.Settings
}

// ResetToBase checkouts baseBranch and hard-resets to origin/baseBranch.
// Must run before Generate so nav.ApplyToFiles reads a clean master baseline;
// otherwise a prior practice's branch (same Actions run) contaminates SUMMARY/index.
func (o *RepoOperator) ResetToBase(repoPath, baseBranch string) error {
	if strings.TrimSpace(baseBranch) == "" {
		return fmt.Errorf("base branch is required")
	}
	if err := run(repoPath, "git", "fetch", "origin"); err != nil {
		return fmt.Errorf("fetch origin: %w", err)
	}
	if err := run(repoPath, "git", "checkout", baseBranch); err != nil {
		return fmt.Errorf("checkout %s: %w", baseBranch, err)
	}
	ref := "origin/" + baseBranch
	if err := run(repoPath, "git", "reset", "--hard", ref); err != nil {
		return fmt.Errorf("reset --hard %s: %w", ref, err)
	}
	if err := run(repoPath, "git", "clean", "-fd"); err != nil {
		return fmt.Errorf("clean -fd: %w", err)
	}
	return nil
}

func (o *RepoOperator) ApplyAndPush(repoPath, branchName, baseBranch string, result *model.GenerateResult, commitMessage string, dryRun bool) (string, error) {
	if err := o.ResetToBase(repoPath, baseBranch); err != nil {
		return "", err
	}

	// -B: create or reset branch from current HEAD (avoids noisy "branch not found" from -D)
	if err := run(repoPath, "git", "checkout", "-B", branchName); err != nil {
		return "", err
	}

	for _, change := range result.Files {
		target := filepath.Join(repoPath, filepath.FromSlash(change.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(target, []byte(change.Content), 0o644); err != nil {
			return "", err
		}
	}

	if err := run(repoPath, "git", "add", "-A"); err != nil {
		return "", err
	}
	dirty, err := runOut(repoPath, "git", "status", "--porcelain")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(dirty) == "" {
		return "", nil
	}

	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=gitbook-practice-synchronization",
		"GIT_AUTHOR_EMAIL=gitbook-practice-synchronization@users.noreply.github.com",
		"GIT_COMMITTER_NAME=gitbook-practice-synchronization",
		"GIT_COMMITTER_EMAIL=gitbook-practice-synchronization@users.noreply.github.com",
	)
	cmd := exec.Command("git", "commit", "-m", commitMessage)
	cmd.Dir = repoPath
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	if dryRun {
		return branchName, nil
	}

	if token := o.Settings.CRepoToken; token != "" {
		url := monitor.ToHTTPSURL(o.Settings.CRepo, token)
		if err := run(repoPath, "git", "remote", "set-url", "origin", url); err != nil {
			return "", err
		}
	}
	if err := run(repoPath, "git", "push", "--set-upstream", "origin", branchName, "--force"); err != nil {
		return "", err
	}
	return branchName, nil
}

func run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runOut(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}
