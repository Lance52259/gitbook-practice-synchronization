package gitops_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Lance52259/gitbook-practice-synchronization/internal/config"
	"github.com/Lance52259/gitbook-practice-synchronization/internal/gitops"
	"github.com/Lance52259/gitbook-practice-synchronization/internal/model"
)

func TestApplyAndPushCreatesBranchWithoutDeleteNoise(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	run("init", "-b", "master")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
	// bare remote for pull/fetch
	remote := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", remote)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bare: %v %s", err, out)
	}
	run("remote", "add", "origin", remote)
	run("push", "-u", "origin", "master")

	op := &gitops.RepoOperator{Settings: &config.Settings{}}
	result := &model.GenerateResult{
		PracticeID: "examples/ecs/basic",
		Files: []model.DocFileChange{
			{Path: "docs/zh-cn/best-practices/ecs/basic.md", Content: "# ok\n", Action: "create"},
		},
	}
	branch, err := op.ApplyAndPush(dir, "gitbook-practice-synchronization/examples-ecs-basic", "master", result, "docs: test", true)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "gitbook-practice-synchronization/examples-ecs-basic" {
		t.Fatalf("branch=%s", branch)
	}
}

func TestResetToBaseClearsPriorPracticeBranch(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	run("init", "-b", "master")
	summary := filepath.Join(dir, "docs", "en-us", "SUMMARY.md")
	if err := os.MkdirAll(filepath.Dir(summary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(summary, []byte("# Summary\n  * [DEW](best-practices/dew/)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
	remote := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("bare: %v %s", err, out)
	}
	run("remote", "add", "origin", remote)
	run("push", "-u", "origin", "master")

	op := &gitops.RepoOperator{Settings: &config.Settings{}}
	// Simulate first practice leaving worktree on feature branch with DDS baked into SUMMARY.
	dds := &model.GenerateResult{
		Files: []model.DocFileChange{{
			Path: "docs/en-us/SUMMARY.md",
			Content: "# Summary\n  * [DDS](best-practices/dds/)\n    * [Introduction](best-practices/dds/index.md)\n  * [DEW](best-practices/dew/)\n",
		}},
	}
	if _, err := op.ApplyAndPush(dir, "gitbook-practice-synchronization/examples-dds-eip", "master", dds, "docs(dds): eip", true); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(summary)
	if !strings.Contains(string(got), "DDS") {
		t.Fatalf("expected DDS on feature branch, got %s", got)
	}

	if err := op.ResetToBase(dir, "master"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(summary)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "DDS") {
		t.Fatalf("after ResetToBase SUMMARY must match master, got:\n%s", got)
	}
	if !strings.Contains(string(got), "DEW") {
		t.Fatalf("master content missing:\n%s", got)
	}
}
