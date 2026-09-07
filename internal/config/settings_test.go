package config

import (
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

func TestApplyEnvSetsRepos(t *testing.T) {
	t.Setenv("B_REPO", "acme/examples-src")
	t.Setenv("C_REPO", "acme/docs-target")
	s := &Settings{}
	applyEnv(s)
	if s.BRepo != "acme/examples-src" || s.CRepo != "acme/docs-target" {
		t.Fatalf("got B=%q C=%q", s.BRepo, s.CRepo)
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
	// Settings filled but env empty → still fail (env is mandatory).
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
