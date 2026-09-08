package mapping

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chnsz/gitbook-practice-synchronization/internal/config"
	"github.com/chnsz/gitbook-practice-synchronization/internal/model"
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

// PR #208 regression: examples/dew/kps-keypair must map to existing dew/keypair.md
// (H1 Deploy Keypair), not create kps_keypair.md / key_pair.md.
func TestIsSyncedKpsKeypairMatchesKeypair(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "zh-cn", "best-practices", "dew")
	if err := mkdirWrite(dir, "keypair.md", "# Deploy Keypair\n"); err != nil {
		t.Fatal(err)
	}
	if err := mkdirWrite(dir, "kms_key.md", "# Deploy KMS Key\n"); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(config.MappingConfig{}, "docs/zh-cn/best-practices")
	if err := r.IndexDocsRoot(root); err != nil {
		t.Fatal(err)
	}

	p := model.Practice{PracticeID: "examples/dew/kps-keypair", SourcePath: "examples/dew/kps-keypair"}
	if !r.IsSynced(p) {
		t.Fatal("kps-keypair should be treated as already synced to keypair.md")
	}
	got := r.Resolve(p)
	if got.Slug != "keypair" || got.Service != "dew" {
		t.Fatalf("Resolve=%+v want dew/keypair", got)
	}
}

func TestResolveKpsKeypairAlias(t *testing.T) {
	r := NewResolver(config.MappingConfig{
		PracticeAliases: map[string]string{"examples/dew/kps-keypair": "keypair"},
	}, "docs/zh-cn/best-practices")
	got := r.Resolve(model.Practice{PracticeID: "examples/dew/kps-keypair"})
	if got.Slug != "keypair" {
		t.Fatalf("slug=%s", got.Slug)
	}
}

func TestSlugCandidatesDropsShortProductPrefix(t *testing.T) {
	cands := SlugCandidates("dew", "kps-keypair")
	found := false
	for _, c := range cands {
		if NormalizeKey(c) == "keypair" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("candidates=%v missing keypair", cands)
	}
	// Must not collapse kms_key → key (remaining too short)
	for _, c := range SlugCandidates("dew", "kms_key") {
		if NormalizeKey(c) == "key" {
			t.Fatalf("kms_key must not yield key: %v", SlugCandidates("dew", "kms_key"))
		}
	}
}

func mkdirWrite(dir, name, content string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}
