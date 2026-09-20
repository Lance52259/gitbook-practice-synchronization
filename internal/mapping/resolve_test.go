package mapping

import (
	"os"
	"path/filepath"
	"strings"
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

// PR #261 regression: B examples/ecs/attached-volume already documented as instance_with_volume.md
// (Reference link points at the same examples path).
func TestIsSyncedViaSourceCodeReference(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "zh-cn", "best-practices", "ecs")
	body := `# 部署挂载数据盘的实例

## 参考信息

- [ECS挂载数据盘最佳实践源码参考](https://github.com/huaweicloud/terraform-provider-huaweicloud/tree/master/examples/ecs/attached-volume)
`
	if err := mkdirWrite(dir, "instance_with_volume.md", body); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(config.MappingConfig{}, "docs/zh-cn/best-practices")
	if err := r.IndexDocsRoot(root); err != nil {
		t.Fatal(err)
	}

	p := model.Practice{PracticeID: "examples/ecs/attached-volume", SourcePath: "examples/ecs/attached-volume"}
	if !r.IsSynced(p) {
		t.Fatal("attached-volume must be synced via Reference source link to instance_with_volume.md")
	}
	got := r.Resolve(p)
	if got.Service != "ecs" || got.Slug != "instance_with_volume" {
		t.Fatalf("Resolve=%+v want ecs/instance_with_volume", got)
	}
}

func TestIsSyncedAttachedVolumeAliasAndRenameCandidate(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "zh-cn", "best-practices", "ecs")
	if err := mkdirWrite(dir, "instance_with_volume.md", "# 部署\n"); err != nil {
		t.Fatal(err)
	}
	r := NewResolver(config.MappingConfig{
		PracticeAliases: map[string]string{"examples/ecs/attached-volume": "instance_with_volume"},
	}, "docs/zh-cn/best-practices")
	if err := r.IndexDocsRoot(root); err != nil {
		t.Fatal(err)
	}
	p := model.Practice{PracticeID: "examples/ecs/attached-volume"}
	if !r.IsSynced(p) {
		t.Fatal("alias/rename candidate should mark attached-volume synced")
	}
}

func TestSlugCandidatesAttachedVolumeRename(t *testing.T) {
	cands := SlugCandidates("ecs", "attached-volume")
	found := false
	for _, c := range cands {
		if NormalizeKey(c) == "instancewithvolume" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("candidates=%v missing instance_with_volume", cands)
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

func TestResolveKeepsNestedDMSKafkaAndIgnoresFlatDuplicate(t *testing.T) {
	root := t.TempDir()
	flat := filepath.Join(root, "docs", "zh-cn", "best-practices", "dms")
	nested := filepath.Join(flat, "kafka")
	if err := mkdirWrite(flat, "instance_configuration.md", "# flat Path A\n"); err != nil {
		t.Fatal(err)
	}
	body := `# Deploy Kafka Instance Configuration

## Reference

- [source](https://github.com/huaweicloud/terraform-provider-huaweicloud/tree/master/examples/dms/kafka/instance-configuration)
`
	if err := mkdirWrite(nested, "instance_configuration.md", body); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(config.MappingConfig{}, "docs/zh-cn/best-practices")
	if err := r.IndexDocsRoot(root); err != nil {
		t.Fatal(err)
	}
	p := model.Practice{PracticeID: "examples/dms/kafka/instance-configuration"}
	if !r.IsSynced(p) {
		t.Fatal("nested KEEP doc must count as synced")
	}
	got := r.Resolve(p)
	if got.Service != "dms" || got.Slug != "kafka/instance_configuration" {
		t.Fatalf("Resolve=%+v want dms/kafka/instance_configuration", got)
	}
	if got.RelPath != "docs/zh-cn/best-practices/dms/kafka/instance_configuration.md" {
		t.Fatalf("RelPath=%s", got.RelPath)
	}
}

func TestResolvePathBAliasesForDuplicateTable(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		id, svc, file, slug string
	}{
		{"examples/dns/zone", "dns", "public_zone.md", "public_zone"},
		{"examples/ecs/attached-volume", "ecs", "instance_with_volume.md", "instance_with_volume"},
		{"examples/ecs/attached-interface", "ecs", "instance_with_interface.md", "instance_with_interface"},
		{"examples/dcs/redis-high-availability-instance", "dcs", "redis_high_availability_instance.md", "redis_high_availability_instance"},
		{"examples/ddm/ddm-account", "ddm", "ddm_account.md", "ddm_account"},
		{"examples/eg/event-subscriptions/custom", "eg", "event_subscription_custom_to_eg.md", "event_subscription_custom_to_eg"},
	}
	cfg := config.MappingConfig{PracticeAliases: map[string]string{
		"examples/dns/zone":                   "public_zone",
		"examples/ecs/attached-volume":        "instance_with_volume",
		"examples/ecs/attached-interface":     "instance_with_interface",
		"redis_ha_instance":                   "redis_high_availability_instance",
		"examples/ddm/ddm-account":            "ddm_account",
		"examples/eg/event-subscriptions/custom": "event_subscription_custom_to_eg",
	}}
	for _, tc := range cases {
		dir := filepath.Join(root, "docs", "zh-cn", "best-practices", tc.svc)
		// Also plant oversimplified Path A where applicable
		switch tc.svc {
		case "dns":
			_ = mkdirWrite(dir, "zone.md", "# Path A\n")
		case "ecs":
			if strings.Contains(tc.file, "volume") {
				_ = mkdirWrite(dir, "attached_volume.md", "# Path A\n")
			} else {
				_ = mkdirWrite(dir, "attached_interface.md", "# Path A\n")
			}
		case "dcs":
			_ = mkdirWrite(dir, "redis_ha_instance.md", "# Path A\n")
		case "ddm":
			_ = mkdirWrite(dir, "account.md", "# Path A\n")
		case "eg":
			_ = mkdirWrite(dir, "custom.md", "# Path A\n")
		}
		if err := mkdirWrite(dir, tc.file, "# KEEP\n"); err != nil {
			t.Fatal(err)
		}
	}
	r := NewResolver(cfg, "docs/zh-cn/best-practices")
	if err := r.IndexDocsRoot(root); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		p := model.Practice{PracticeID: tc.id}
		if !r.IsSynced(p) {
			t.Fatalf("%s should be synced to KEEP", tc.id)
		}
		got := r.Resolve(p)
		if got.Service != tc.svc || got.Slug != tc.slug {
			t.Fatalf("%s Resolve=%+v want %s/%s", tc.id, got, tc.svc, tc.slug)
		}
	}
}

func TestFlatPathAAloneDoesNotSyncNestedCanonical(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "zh-cn", "best-practices", "dms")
	if err := mkdirWrite(dir, "instance_configuration.md", "# Path A only\n"); err != nil {
		t.Fatal(err)
	}
	r := NewResolver(config.MappingConfig{}, "docs/zh-cn/best-practices")
	if err := r.IndexDocsRoot(root); err != nil {
		t.Fatal(err)
	}
	p := model.Practice{PracticeID: "examples/dms/kafka/instance-configuration"}
	if r.IsSynced(p) {
		t.Fatal("flat Path A must not mark nested KEEP target as synced")
	}
	got := r.Resolve(p)
	if got.Slug != "kafka/instance_configuration" {
		t.Fatalf("slug=%s", got.Slug)
	}
}

func TestSlugCandidatesAvailabilityAbbrev(t *testing.T) {
	cands := SlugCandidates("dcs", "redis-high-availability-instance")
	found := false
	for _, c := range cands {
		if NormalizeKey(c) == "redishainstance" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("candidates=%v missing redis_ha_instance", cands)
	}
}

func mkdirWrite(dir, name, content string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}
