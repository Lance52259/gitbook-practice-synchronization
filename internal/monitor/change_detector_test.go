package monitor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chnsz/gitbook-practice-synchronization/internal/config"
	"github.com/chnsz/gitbook-practice-synchronization/internal/model"
	"github.com/chnsz/gitbook-practice-synchronization/internal/monitor"
)

func makeExamples(t *testing.T, root string) {
	t.Helper()
	_ = os.MkdirAll(filepath.Join(root, "examples", "alpha"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "examples", "alpha", "README.md"), []byte("# Alpha Practice\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(root, "examples", "beta"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "examples", "beta", "main.sh"), []byte("echo ok\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "examples", "README.md"), []byte("ignore me\n"), 0o644)
}

func TestEnumeratePractices(t *testing.T) {
	root := t.TempDir()
	makeExamples(t, root)
	practices, err := monitor.EnumeratePractices(filepath.Join(root, "examples"), "examples", []string{"README.md", "README", ".gitkeep"}, "directory")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, p := range practices {
		ids[p.PracticeID] = true
	}
	if !ids["examples/alpha"] || !ids["examples/beta"] || len(practices) != 2 {
		t.Fatalf("unexpected practices: %+v", practices)
	}
	for _, p := range practices {
		if p.PracticeID == "examples/alpha" && p.Title != "Alpha Practice" {
			t.Fatalf("title=%q", p.Title)
		}
	}
}

func TestEnumerateNestedPractices(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "examples", "ecs", "basic"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "examples", "ecs", "basic", "main.tf"), []byte("resource \"huaweicloud_vpc\" \"t\" {}\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(root, "examples", "vpc", "peering"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "examples", "vpc", "peering", "main.tf"), []byte(""), 0o644)

	practices, err := monitor.EnumeratePractices(filepath.Join(root, "examples"), "examples", []string{"README.md"}, "nested_directory")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, p := range practices {
		ids[p.PracticeID] = true
	}
	if !ids["examples/ecs/basic"] || !ids["examples/vpc/peering"] {
		t.Fatalf("got %+v", practices)
	}
}

func TestDetectHybrid(t *testing.T) {
	tmp := t.TempDir()
	b := filepath.Join(tmp, "b")
	c := filepath.Join(tmp, "c")
	makeExamples(t, b)
	_ = os.MkdirAll(filepath.Join(c, "docs", "best-practices"), 0o755)
	_ = os.WriteFile(filepath.Join(c, "docs", "best-practices", "alpha.md"), []byte("# alpha\n"), 0o644)
	manifest, _ := json.Marshal(map[string]any{"practices": []string{"examples/beta"}})
	_ = os.WriteFile(filepath.Join(c, "synced-practices.json"), manifest, 0o644)

	s := &config.Settings{
		BRepo:           "o/b",
		CRepo:           "o/c",
		SyncedStrategy:  "hybrid",
		CDocsRoot:       "docs/best-practices",
		CSyncedManifest: "synced-practices.json",
		BExamplesPath:   "examples",
		IgnoreNames:     []string{"README.md", "README", ".gitkeep"},
		Granularity:     "directory",
	}
	ctx := &monitor.RepoContext{
		B: model.RepoRef{Name: "o/b", LocalPath: b, CommitSHA: "b1"},
		C: model.RepoRef{Name: "o/c", LocalPath: c, CommitSHA: "c1"},
	}
	result, err := (&monitor.ChangeDetector{Settings: s}).Detect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.NewPractices) != 0 {
		t.Fatalf("expected no new, got %+v", result.NewPractices)
	}
}

func TestEnumerateDeepNestedPractices(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "examples", "aom", "alarm-rule")
	_ = os.MkdirAll(filepath.Join(base, "distribute-alarm"), 0o755)
	_ = os.WriteFile(filepath.Join(base, "distribute-alarm", "main.tf"), []byte(""), 0o644)
	_ = os.MkdirAll(filepath.Join(base, "prevent-elb-alarm-storm"), 0o755)
	_ = os.WriteFile(filepath.Join(base, "prevent-elb-alarm-storm", "main.tf"), []byte(""), 0o644)

	practices, err := monitor.EnumeratePractices(filepath.Join(root, "examples"), "examples", []string{"README.md"}, "nested_directory")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, p := range practices {
		ids[p.PracticeID] = true
	}
	if ids["examples/aom/alarm-rule"] {
		t.Fatalf("container should not be enumerated: %+v", practices)
	}
	if !ids["examples/aom/alarm-rule/distribute-alarm"] || !ids["examples/aom/alarm-rule/prevent-elb-alarm-storm"] {
		t.Fatalf("got %+v", practices)
	}
}

func TestDetectServiceAlias(t *testing.T) {
	tmp := t.TempDir()
	b := filepath.Join(tmp, "b")
	c := filepath.Join(tmp, "c")
	_ = os.MkdirAll(filepath.Join(b, "examples", "antiddos", "basic"), 0o755)
	_ = os.WriteFile(filepath.Join(b, "examples", "antiddos", "basic", "main.tf"), []byte(""), 0o644)
	_ = os.MkdirAll(filepath.Join(b, "examples", "antiddos", "default-protection-policy"), 0o755)
	_ = os.WriteFile(filepath.Join(b, "examples", "antiddos", "default-protection-policy", "main.tf"), []byte(""), 0o644)
	_ = os.MkdirAll(filepath.Join(c, "docs", "zh-cn", "best-practices", "anti-ddos"), 0o755)
	_ = os.WriteFile(filepath.Join(c, "docs", "zh-cn", "best-practices", "anti-ddos", "basic.md"), []byte("#\n"), 0o644)
	_ = os.WriteFile(filepath.Join(c, "docs", "zh-cn", "best-practices", "anti-ddos", "default_protection_policy.md"), []byte("#\n"), 0o644)

	s := &config.Settings{
		BRepo:          "o/b",
		CRepo:          "o/c",
		SyncedStrategy: "path_infer",
		CDocsRoot:      "docs/zh-cn/best-practices",
		BExamplesPath:  "examples",
		IgnoreNames:    []string{"README.md"},
		Granularity:    "nested_directory",
		Mapping: config.MappingConfig{
			ServiceAliases: map[string]string{"antiddos": "anti-ddos"},
		},
	}
	ctx := &monitor.RepoContext{
		B: model.RepoRef{LocalPath: b},
		C: model.RepoRef{LocalPath: c},
	}
	result, err := (&monitor.ChangeDetector{Settings: s}).Detect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.NewPractices) != 0 {
		t.Fatalf("expected all synced via alias, got %+v", result.NewPractices)
	}
}

func TestDetectNewOnly(t *testing.T) {
	tmp := t.TempDir()
	b := filepath.Join(tmp, "b")
	c := filepath.Join(tmp, "c")
	makeExamples(t, b)
	_ = os.MkdirAll(filepath.Join(c, "docs", "best-practices"), 0o755)

	s := &config.Settings{
		BRepo:          "o/b",
		CRepo:          "o/c",
		SyncedStrategy: "path_infer",
		CDocsRoot:      "docs/best-practices",
		BExamplesPath:  "examples",
		IgnoreNames:    []string{"README.md", "README", ".gitkeep"},
		Granularity:    "directory",
	}
	ctx := &monitor.RepoContext{
		B: model.RepoRef{LocalPath: b},
		C: model.RepoRef{LocalPath: c},
	}
	result, err := (&monitor.ChangeDetector{Settings: s}).Detect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.NewPractices) != 2 {
		t.Fatalf("got %d new", len(result.NewPractices))
	}
}
