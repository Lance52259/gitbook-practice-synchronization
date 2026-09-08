package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chnsz/gitbook-practice-synchronization/internal/ai/provider"
	"github.com/chnsz/gitbook-practice-synchronization/internal/model"
)

const outputSchemaHint = `你必须只输出一个 JSON 对象（不要 Markdown 围栏），格式如下：
{
  "practice_id": "<与输入相同的 practice_id>",
  "summary": "<one-sentence English summary for the PR description>",
  "files": [
    {
      "path": "<相对 C 仓库根目录的路径>",
      "action": "create",
      "content": "<完整 Markdown 文件内容>"
    }
  ]
}
约束：
1. path 不得包含 ..，且必须落在 docs/zh-cn/ 或 docs/en-us/ 下
2. content 为完整文档正文
3. 不要在 JSON 外输出任何文字
4. 必须同时 create 中文与英文实践正文（同一 {service}/{practice}）
5. 新服务时还须 create 中英 index.md，且须达到 Skill「分类 index.md 规范」/Anti-DDoS 同等篇幅（What is≥2段、Overview两段套话、List含导语与条目说明且标题与正文 H1 一致为 Deploy…/部署…、参考资料含 Terraform Official Documentation）；禁止精简版。列表一句话须用：EN Introduces how to use Terraform to automatically {H1首字母小写}, including …；ZH 介绍如何使用Terraform自动化{H1}，包括…；禁止 automate «Deploy …» / 自动化完成「部署…」弱占位
6. 正文 H1：中文「部署…」、英文「Deploy …」；无 data source 时导语勿写「资源和数据源」；tfvars 步骤标题含（可选）/(Optional)；步骤内联 variable 且同一变量名全文只声明一次（后步禁止重复）；无 region 参数用完整缺省继承注释
7. SUMMARY/README 导航由编排层按「英文定序、中文跟随」补丁，禁止整文件重写 SUMMARY
8. 严禁把 SUMMARY.md 重写成短目录或改掉「# Summary」标题`

// PromptTemplates assembles chat messages for generation.
type PromptTemplates struct {
	TemplatesDir string
}

func (p *PromptTemplates) LoadTemplate(name string) (string, error) {
	path := filepath.Join(p.TemplatesDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("template not found: %s: %w", path, err)
	}
	return string(data), nil
}

// BuildMessagesInput carries generation prompt inputs.
type BuildMessagesInput struct {
	Skill         *Skill
	Practice      model.Practice
	SourceContext string
	TargetPath    string
	TemplateName  string
	DocsRoot      string
	// NavBaselines maps relative path → current C-repo file content.
	NavBaselines map[string]string
}

func (p *PromptTemplates) BuildMessages(in BuildMessagesInput) ([]provider.ChatMessage, error) {
	template, err := p.LoadTemplate(in.TemplateName)
	if err != nil {
		return nil, err
	}
	categoryZH, _ := p.LoadTemplate("category_index_template.md")
	categoryEN, _ := p.LoadTemplate("category_index_template_en.md")
	categoryBlock := formatCategoryTemplates(categoryZH, categoryEN)

	refs, _ := in.Skill.ReadReferences(8)
	var refParts []string
	for _, r := range refs {
		refParts = append(refParts, "### reference:"+r[0]+"\n"+r[1])
	}
	refText := strings.Join(refParts, "\n\n")
	if refText == "" {
		refText = "(无)"
	}

	baselineText := formatNavBaselines(in.NavBaselines)

	system := "你是文档工程师，根据最佳实践源码与 Skill 约束，为文档仓库生成变更。\n" + outputSchemaHint
	user := fmt.Sprintf(`## Skill: %s
%s

## 目标
- practice_id: %s
- 建议写入路径（中文正文）: %s
- 英文正文对称路径: docs/en-us/best-practices/{service}/{practice}.md
- 处理顺序: 中文正文 → 英文正文 → 英文导航(字母序) → 中文导航(跟随英文)
- C 仓文档根: %s

## 文档模板（请在结构上对齐，可按源内容充实）
%s

## 分类 index.md 模板（新服务 create 时必须对齐 Anti-DDoS 篇幅；禁止精简版）
%s

## 参考资料
%s

## C 仓导航基线（必须保留全部已有内容；禁止整文件重写）
%s

## 源最佳实践上下文
%s
`, in.Skill.Title, in.Skill.Body, in.Practice.PracticeID, in.TargetPath, in.DocsRoot, template, categoryBlock, refText, baselineText, in.SourceContext)

	return []provider.ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}, nil
}

func formatCategoryTemplates(zh, en string) string {
	zh = strings.TrimSpace(zh)
	en = strings.TrimSpace(en)
	if zh == "" && en == "" {
		return "(无本地分类模板；严格按 Skill「分类 index.md 规范」与 anti-ddos/index.md 生成。)"
	}
	var b strings.Builder
	if zh != "" {
		b.WriteString("### category_index_template.md (ZH)\n```markdown\n")
		b.WriteString(zh)
		if !strings.HasSuffix(zh, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
	}
	if en != "" {
		b.WriteString("\n### category_index_template_en.md (EN)\n```markdown\n")
		b.WriteString(en)
		if !strings.HasSuffix(en, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
	}
	return b.String()
}

func formatNavBaselines(baselines map[string]string) string {
	if len(baselines) == 0 {
		return "(未提供基线：请勿输出 SUMMARY.md / README.md / 已有 index.md 的 update；仅输出实践正文 create。)"
	}
	var keys []string
	for k := range baselines {
		keys = append(keys, k)
	}
	// stable-ish order
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	var b strings.Builder
	b.WriteString("以下为 C 仓**当前真实文件**。若你输出对应 path 的 update，必须以这些全文为起点，只插入本实践相关新行，不得删除任何已有行。\n")
	const maxPerFile = 120000
	for _, k := range keys {
		content := baselines[k]
		if len(content) > maxPerFile {
			content = content[:maxPerFile] + "\n…(truncated)…\n"
		}
		b.WriteString("\n### baseline:")
		b.WriteString(k)
		b.WriteString("\n```markdown\n")
		b.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
	}
	return b.String()
}
