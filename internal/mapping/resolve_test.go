package mapping

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Lance52259/gitbook-practice-synchronization/internal/config"
	"github.com/Lance52259/gitbook-practice-synchronization/internal/model"
)

func TestResolveAntiddosAlias(t *testing.T) {
	r := NewResolver(config.MappingConfig{
		ServiceAliases: map[string]string{"antiddos": "anti-ddos"},
	}, "docs/zh-cn/best-practices")
	p := model.Practice{PracticeID: "examples/antiddos/basic", SourcePath: "examples/antiddos/basic"}
	got := r.Resolve(p)
	if got.Service != "anti-ddos" || got.Slug != "basic" {
		t.Fatalf("got %+v", got)
	}
	if got.RelPath != "docs/zh-cn/best-practices/anti-ddos/basic.md" {
		t.Fatalf("path=%s", got.RelPath)
	}
}

func TestResolveSlugNormalizeAndAlias(t *testing.T) {
	r := NewResolver(config.MappingConfig{
		ServiceAliases:  map[string]string{"antiddos": "anti-ddos"},
		PracticeAliases: map[string]string{"cbr/vault-server": "server_vault"},
	}, "docs/zh-cn/best-practices")

	p := model.Practice{PracticeID: "examples/antiddos/default-protection-policy"}
	got := r.Resolve(p)
	if got.Slug != "default_protection_policy" {
		t.Fatalf("slug=%s", got.Slug)
	}

	p2 := model.Practice{PracticeID: "examples/cbr/vault-server"}
	got2 := r.Resolve(p2)
	if got2.Slug != "server_vault" {
		t.Fatalf("slug=%s", got2.Slug)
	}
}

func TestIsSyncedFuzzyIndex(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "zh-cn", "best-practices", "anti-ddos")
	if err := mkdirWrite(dir, "basic.md", "# basic\n"); err != nil {
		t.Fatal(err)
	}
	if err := mkdirWrite(dir, "default_protection_policy.md", "# pol\n"); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(config.MappingConfig{
		ServiceAliases: map[string]string{"antiddos": "anti-ddos"},
	}, "docs/zh-cn/best-practices")
	if err := r.IndexDocsRoot(root); err != nil {
		t.Fatal(err)
	}

	if !r.IsSynced(model.Practice{PracticeID: "examples/antiddos/basic"}) {
		t.Fatal("basic should be synced")
	}
	if !r.IsSynced(model.Practice{PracticeID: "examples/antiddos/default-protection-policy"}) {
		t.Fatal("default-protection-policy should match default_protection_policy.md")
	}
	if r.IsSynced(model.Practice{PracticeID: "examples/antiddos/missing"}) {
		t.Fatal("missing should not be synced")
	}
}

func mkdirWrite(dir, name, content string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}
