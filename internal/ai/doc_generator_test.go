package ai_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Lance52259/gitbook-practice-synchronization/internal/ai"
	"github.com/Lance52259/gitbook-practice-synchronization/internal/config"
	"github.com/Lance52259/gitbook-practice-synchronization/internal/model"
)

func TestExtractJSON(t *testing.T) {
	m, err := ai.ExtractJSON(`{"a": 1}`)
	if err != nil || int(m["a"].(float64)) != 1 {
		t.Fatalf("%v %v", m, err)
	}
	m, err = ai.ExtractJSON("```json\n{\"a\": 2}\n```")
	if err != nil || int(m["a"].(float64)) != 2 {
		t.Fatalf("%v %v", m, err)
	}
	// preamble + balanced object
	m, err = ai.ExtractJSON("Here is the result:\n{\"a\": 3, \"b\": \"x\"}\nThanks.")
	if err != nil || int(m["a"].(float64)) != 3 {
		t.Fatalf("preamble: %v %v", m, err)
	}
	// trailing comma repair
	m, err = ai.ExtractJSON(`{"a": 4, "files": [{"path": "x.md", "content": "hi",},],}`)
	if err != nil || int(m["a"].(float64)) != 4 {
		t.Fatalf("trailing comma: %v %v", m, err)
	}
	// smart quotes
	m, err = ai.ExtractJSON("{\u201ca\u201d: 5}")
	if err != nil || int(m["a"].(float64)) != 5 {
		t.Fatalf("smart quotes: %v %v", m, err)
	}
	if _, err := ai.ExtractJSON(""); err == nil {
		t.Fatal("expected empty error")
	}
	if _, err := ai.ExtractJSON("not json at all"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestValidatePaths(t *testing.T) {
	err := ai.ValidatePaths([]model.DocFileChange{{Path: "docs/best-practices/x.md", Content: "hi", Action: "create"}}, []string{"docs/"})
	if err != nil {
		t.Fatal(err)
	}
	err = ai.ValidatePaths([]model.DocFileChange{{Path: "etc/passwd", Content: "x", Action: "create"}}, []string{"docs/"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveTargetPath(t *testing.T) {
	s := &config.Settings{
		SkillID:   "best-practice-doc",
		CDocsRoot: "docs/zh-cn/best-practices",
		Mapping: config.MappingConfig{
			Defaults: config.MappingDefaults{
				TargetPathPattern: "docs/zh-cn/best-practices/{service}/{practice_slug}.md",
				SkillID:           "best-practice-doc",
				Template:          "best_practice_template.md",
			},
			ServiceAliases: map[string]string{"antiddos": "anti-ddos"},
		},
	}
	p := model.Practice{PracticeID: "examples/ecs/foo-bar", SourcePath: "examples/ecs/foo-bar"}
	path, skill, template := ai.ResolveTargetPath(s, p)
	if path != "docs/zh-cn/best-practices/ecs/foo_bar.md" || skill != "best-practice-doc" || template == "" {
		t.Fatalf("%s %s %s", path, skill, template)
	}
	p2 := model.Practice{PracticeID: "examples/antiddos/basic"}
	path2, _, _ := ai.ResolveTargetPath(s, p2)
	if path2 != "docs/zh-cn/best-practices/anti-ddos/basic.md" {
		t.Fatalf("antiddos path=%s", path2)
	}
}

func TestPackSourceContext(t *testing.T) {
	d := filepath.Join(t.TempDir(), "examples", "foo")
	_ = os.MkdirAll(d, 0o755)
	_ = os.WriteFile(filepath.Join(d, "README.md"), []byte("# Hello\n"), 0o644)
	_ = os.WriteFile(filepath.Join(d, "run.sh"), []byte("echo 1\n"), 0o644)
	text, err := ai.PackSourceContext(d, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(text, "file:README.md") || !contains(text, "file:run.sh") {
		t.Fatalf("%s", text)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()))
}
