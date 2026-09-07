package gitops_test

import (
	"strings"
	"testing"

	"github.com/chnsz/gitbook-practice-synchronization/internal/gitops"
	"github.com/chnsz/gitbook-practice-synchronization/internal/model"
)

func TestBuildPRBody(t *testing.T) {
	p := model.Practice{
		PracticeID: "examples/rds/basic_instance",
		SourcePath: "examples/rds/basic_instance",
	}
	result := &model.GenerateResult{
		PracticeID: p.PracticeID,
		Summary:    "Add bilingual RDS basic instance docs",
		Files: []model.DocFileChange{
			{Path: "docs/zh-cn/best-practices/rds/basic_instance.md", Content: "# x\n", Action: "create"},
			{Path: "docs/en-us/best-practices/rds/basic_instance.md", Content: "# x\n", Action: "create"},
			{Path: "docs/zh-cn/SUMMARY.md", Content: "...", Action: "update"},
		},
	}
	body := gitops.BuildPRBody(gitops.PRBodyInput{
		Practice:   p,
		Result:     result,
		BRepo:      "org/source-examples",
		CRepo:      "org/docs-target",
		BSHA:       "abc1234",
		SkillID:    "best-practice-doc",
		AIProvider: "DeepSeek",
	})
	needles := []string{
		"| Service | `rds` |",
		"| Practice | `basic instance` |",
		"`examples/rds/basic_instance`",
		"org/source-examples@abc1234",
		"| Target repo (C) | `org/docs-target` |",
		"source repo **B** (`org/source-examples`)",
		"target repo **C** (`org/docs-target`)",
		"**bilingual**",
		"**Chinese and English**",
		"Add bilingual RDS basic instance docs",
		"`create` `docs/zh-cn/best-practices/rds/basic_instance.md`",
		"## Review checklist",
		"docs(rds): support new best practice for basic instance",
		"English body:",
		"Skill `best-practice-doc` + `DeepSeek`",
		"Best Practice Source Code Reference For {ServiceName} {PracticeObject}",
		"gitbook-practice-synchronization",
	}
	for _, n := range needles {
		if !strings.Contains(body, n) {
			t.Fatalf("missing %q in body:\n%s", n, body)
		}
	}
	for _, bad := range []string{
		"into Chinese docs",
		"hcbp-demo conventions",
		"repo C (hcbp-demo)",
	} {
		if strings.Contains(body, bad) {
			t.Fatalf("stale wording %q still present:\n%s", bad, body)
		}
	}
}

func TestSummarizeChanges(t *testing.T) {
	result := &model.GenerateResult{
		PracticeID: "examples/foo",
		Summary:    "Add foo docs",
		Files: []model.DocFileChange{
			{Path: "docs/best-practices/foo.md", Content: "# foo\n", Action: "create"},
		},
	}
	body := gitops.SummarizeChanges(result, "acme/examples", "abc")
	if !strings.Contains(body, "examples/foo") || !strings.Contains(body, "docs/best-practices/foo.md") {
		t.Fatalf("%s", body)
	}
}
