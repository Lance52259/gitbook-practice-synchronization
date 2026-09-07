package config

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRequireReposRequiresEnv(t *testing.T) {
	t.Setenv("B_REPO", "")
	t.Setenv("C_REPO", "")
	s := &Settings{BRepo: "", CRepo: ""}
	err := s.RequireRepos()
	if err == nil {
		t.Fatal("expected error when B_REPO/C_REPO unset")
	}
	if !strings.Contains(err.Error(), "B_REPO") || !strings.Contains(err.Error(), "C_REPO") {
		t.Fatalf("error should mention both vars: %v", err)
	}

	t.Setenv("B_REPO", "owner/b-repo")
	t.Setenv("C_REPO", "owner/c-repo")
	s.BRepo = "owner/b-repo"
	s.CRepo = "owner/c-repo"
	if err := s.RequireRepos(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestRequireReposRejectsInvalidFormat(t *testing.T) {
	t.Setenv("B_REPO", "not-a-repo")
	t.Setenv("C_REPO", "owner/c-repo")
	s := &Settings{BRepo: "not-a-repo", CRepo: "owner/c-repo"}
	err := s.RequireRepos()
	if err == nil || !strings.Contains(err.Error(), "B_REPO") {
		t.Fatalf("expected B_REPO format error, got %v", err)
	}
}

func TestValidateRepoRef(t *testing.T) {
	ok := []string{
		"owner/repo",
		"huaweicloud/terraform-provider-huaweicloud",
		"https://github.com/chnsz/hcbp-demo",
		"https://github.com/chnsz/hcbp-demo.git",
	}
	for _, v := range ok {
		if err := ValidateRepoRef(v); err != nil {
			t.Fatalf("%q: %v", v, err)
		}
	}
	bad := []string{"", "noslash", "a/b/c", "http://example.com/x/y"}
	for _, v := range bad {
		if err := ValidateRepoRef(v); err == nil {
			t.Fatalf("%q should be invalid", v)
		}
	}
}

func TestRequireCRepoTokenForPush(t *testing.T) {
	s := &Settings{}
	if err := s.RequireCRepoTokenForPush(); err == nil {
		t.Fatal("expected error")
	}
	s.CRepoToken = "tok"
	if err := s.RequireCRepoTokenForPush(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyEnvSetsRepos(t *testing.T) {
	t.Setenv("B_REPO", "acme/examples-src")
	t.Setenv("C_REPO", "acme/docs-target")
	s := &Settings{}
	applyEnv(s)
	if s.BRepo != "acme/examples-src" || s.CRepo != "acme/docs-target" {
		t.Fatalf("got B=%q C=%q", s.BRepo, s.CRepo)
	}
}

func TestApplyEnvWarnsInvalidInt(t *testing.T) {
	t.Setenv("AI_MAX_TOKENS", "not-a-number")
	s := &Settings{AIMaxTokens: 123}
	var buf bytes.Buffer
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	applyEnv(s)
	_ = w.Close()
	os.Stderr = old
	_, _ = buf.ReadFrom(r)
	if s.AIMaxTokens != 123 {
		t.Fatalf("should keep default, got %d", s.AIMaxTokens)
	}
	if !strings.Contains(buf.String(), "AI_MAX_TOKENS") {
		t.Fatalf("expected warn on stderr, got %q", buf.String())
	}
}

func TestApplyFileConfigIgnoresRepo(t *testing.T) {
	s := &Settings{}
	fc := &fileConfig{}
	fc.Repos.B.Repo = "yaml/should-ignore-b"
	fc.Repos.C.Repo = "yaml/should-ignore-c"
	fc.Repos.B.ExamplesPath = "examples"
	applyFileConfig(s, fc)
	if s.BRepo != "" || s.CRepo != "" {
		t.Fatalf("YAML repo must be ignored, got B=%q C=%q", s.BRepo, s.CRepo)
	}
	if s.BExamplesPath != "examples" {
		t.Fatalf("examples_path should still apply: %q", s.BExamplesPath)
	}
}

func TestRequireReposRejectsSettingsWithoutEnv(t *testing.T) {
	_ = os.Unsetenv("B_REPO")
	_ = os.Unsetenv("C_REPO")
	t.Setenv("B_REPO", "")
	t.Setenv("C_REPO", "")
	s := &Settings{BRepo: "owner/b", CRepo: "owner/c"}
	err := s.RequireRepos()
	if err == nil {
		t.Fatal("expected error when env empty even if Settings populated")
	}
}

func TestHasAIAPIKey(t *testing.T) {
	if (&Settings{}).HasAIAPIKey() {
		t.Fatal("empty should be false")
	}
	if !(&Settings{AIAPIKey: " k "}).HasAIAPIKey() {
		t.Fatal("whitespace-trimmed key should be true")
	}
}
