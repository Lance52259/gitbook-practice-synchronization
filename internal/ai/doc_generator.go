package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chnsz/gitbook-practice-synchronization/internal/ai/provider"
	"github.com/chnsz/gitbook-practice-synchronization/internal/config"
	"github.com/chnsz/gitbook-practice-synchronization/internal/mapping"
	"github.com/chnsz/gitbook-practice-synchronization/internal/model"
	"github.com/chnsz/gitbook-practice-synchronization/internal/nav"
)

var jsonFence = regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")

func ExtractJSON(text string) (map[string]any, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "\ufeff")
	if text == "" {
		return nil, fmt.Errorf("unable to parse JSON from model output: empty response")
	}

	var candidates []string
	if fenced := extractMarkdownJSONFence(text); fenced != "" {
		candidates = append(candidates, fenced)
	}
	candidates = append(candidates, text)
	if obj := extractBalancedObject(text); obj != "" && obj != text {
		candidates = append(candidates, obj)
	}

	var lastErr error
	seen := map[string]bool{}
	for _, c := range candidates {
		for _, attempt := range []string{c, repairCommonJSONIssues(c)} {
			attempt = strings.TrimSpace(attempt)
			if attempt == "" || seen[attempt] {
				continue
			}
			seen[attempt] = true
			var out map[string]any
			if err := json.Unmarshal([]byte(attempt), &out); err != nil {
				lastErr = err
				continue
			}
			return out, nil
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("unable to parse JSON from model output: %w", lastErr)
	}
	return nil, fmt.Errorf("unable to parse JSON from model output")
}

func extractMarkdownJSONFence(text string) string {
	m := jsonFence.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	inner := strings.TrimSpace(m[1])
	if strings.HasPrefix(inner, "{") {
		return inner
	}
	if obj := extractBalancedObject(inner); obj != "" {
		return obj
	}
	return inner
}

// extractBalancedObject returns the first top-level {...} slice using brace depth.
func extractBalancedObject(text string) string {
	start := strings.Index(text, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(text); i++ {
		ch := text[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return ""
}

func repairCommonJSONIssues(s string) string {
	s = strings.ReplaceAll(s, "\u201c", `"`)
	s = strings.ReplaceAll(s, "\u201d", `"`)
	s = strings.ReplaceAll(s, "\u2018", "'")
	s = strings.ReplaceAll(s, "\u2019", "'")
	// trailing commas before } or ]
	reTrailing := regexp.MustCompile(`,\s*([}\]])`)
	s = reTrailing.ReplaceAllString(s, "$1")
	return s
}

// DocGenerator orchestrates Skill + DeepSeek generation.
type DocGenerator struct {
	Settings *config.Settings
	Provider provider.Provider
	Skills   *SkillLoader
	Prompts  *PromptTemplates
}

func NewDocGenerator(settings *config.Settings, p provider.Provider) *DocGenerator {
	return &DocGenerator{
		Settings: settings,
		Provider: p,
		Skills:   &SkillLoader{SkillsRoot: filepath.Join(settings.RepoRoot, settings.SkillRoot)},
		Prompts:  &PromptTemplates{TemplatesDir: settings.TemplatesDir()},
	}
}

// ResolveTargetPath returns target path, skill id, template name.
func ResolveTargetPath(settings *config.Settings, practice model.Practice) (target, skillID, template string) {
	defaults := settings.Mapping.Defaults
	skillID = defaults.SkillID
	if skillID == "" {
		skillID = settings.SkillID
	}
	template = defaults.Template
	if template == "" {
		template = "best_practice_template.md"
	}
	pattern := defaults.TargetPathPattern
	if pattern == "" {
		pattern = "docs/zh-cn/best-practices/{service}/{practice_slug}.md"
	}
	for _, rule := range settings.Mapping.Rules {
		match := strings.TrimRight(rule.Match, "*")
		match = strings.TrimRight(match, "/")
		if match != "" && strings.HasPrefix(practice.PracticeID, match) {
			if rule.SkillID != "" {
				skillID = rule.SkillID
			}
			if rule.Template != "" {
				template = rule.Template
			}
			if rule.TargetPathPattern != "" {
				pattern = rule.TargetPathPattern
			}
			break
		}
	}
	resolver := mapping.NewResolver(settings.Mapping, settings.CDocsRoot)
	doc := resolver.Resolve(practice)
	service := doc.Service
	slug := doc.Slug
	if service == "" {
		service = practice.Slug()
	}
	if slug == "" {
		slug = practice.Slug()
	}
	target = strings.ReplaceAll(pattern, "{practice_slug}", slug)
	target = strings.ReplaceAll(target, "{service}", service)
	target = strings.ReplaceAll(target, "{practice_id}", practice.PracticeID)
	return target, skillID, template
}

// PackSourceContext packs example files into a prompt budget.
func PackSourceContext(practiceDir string, maxChars int) (string, error) {
	st, err := os.Stat(practiceDir)
	if err != nil || !st.IsDir() {
		return fmt.Sprintf("(missing directory: %s)", practiceDir), nil
	}

	priority := map[string]int{"README.md": 0, "README": 0, "readme.md": 0, "SKILL.md": 0, "doc.md": 0}
	type item struct {
		path string
		rel  string
		rank int
	}
	var files []item
	_ = filepath.WalkDir(practiceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".zip", ".tar", ".gz":
			return nil
		}
		rel, _ := filepath.Rel(practiceDir, path)
		rank := 1
		if r, ok := priority[d.Name()]; ok {
			rank = r
		}
		files = append(files, item{path: path, rel: filepath.ToSlash(rel), rank: rank})
		return nil
	})

	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if files[j].rank < files[i].rank || (files[j].rank == files[i].rank && files[j].rel < files[i].rel) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	var parts []string
	used := 0
	for _, f := range files {
		data, err := os.ReadFile(f.path)
		if err != nil {
			continue
		}
		chunk := "### file:" + f.rel + "\n" + string(data) + "\n"
		if used+len(chunk) > maxChars {
			remain := maxChars - used
			if remain < 64 {
				break
			}
			chunk = chunk[:remain] + "\n...[truncated]...\n"
			parts = append(parts, chunk)
			break
		}
		parts = append(parts, chunk)
		used += len(chunk)
	}
	if len(parts) == 0 {
		return "(empty practice directory)", nil
	}
	return strings.Join(parts, "\n"), nil
}

func ValidatePaths(files []model.DocFileChange, allowlist []string) error {
	for _, f := range files {
		path := filepath.ToSlash(strings.TrimLeft(f.Path, "/"))
		if path == "" || strings.Contains(path, "..") {
			return fmt.Errorf("unsafe path: %q", f.Path)
		}
		ok := false
		for _, prefix := range allowlist {
			if strings.HasPrefix(path, prefix) {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("path not in allowlist: %s", path)
		}
	}
	return nil
}

func (g *DocGenerator) Generate(ctx context.Context, practice model.Practice, practiceDir, cRepoRoot string) (*model.GenerateResult, error) {
	targetPath, skillID, templateName := ResolveTargetPath(g.Settings, practice)
	skill, err := g.Skills.Load(skillID)
	if err != nil {
		return nil, err
	}
	sourceContext, err := PackSourceContext(practiceDir, g.Settings.MaxContextChars)
	if err != nil {
		return nil, err
	}

	resolver := mapping.NewResolver(g.Settings.Mapping, g.Settings.CDocsRoot)
	doc := resolver.Resolve(practice)
	baselines := nav.LoadBaselines(cRepoRoot, g.Settings.CDocsRoot, doc.Service)

	messages, err := g.Prompts.BuildMessages(BuildMessagesInput{
		Skill:         skill,
		Practice:      practice,
		SourceContext: sourceContext,
		TargetPath:    targetPath,
		TemplateName:  templateName,
		DocsRoot:      g.Settings.CDocsRoot,
		NavBaselines:  baselines,
	})
	if err != nil {
		return nil, err
	}

	var lastErr error
	var raw string
	var dumpPaths []string
	for attempt := 0; attempt <= g.Settings.AIMaxRetries; attempt++ {
		completion, err := g.Provider.Complete(ctx, messages, 0.2, g.Settings.ResponseFormatJSON)
		if err != nil {
			lastErr = err
			continue
		}
		raw = completion.Content
		data, err := ExtractJSON(raw)
		if err != nil {
			dump := g.dumpAIFailure(practice.PracticeID, attempt, raw, err)
			if dump != "" {
				dumpPaths = append(dumpPaths, dump)
				fmt.Printf("AI JSON parse failed for %s (attempt %d); dumped %s (%d bytes)\n",
					practice.PracticeID, attempt+1, dump, len(raw))
			}
			lastErr = err
			// Ask model to emit a complete JSON object on the next try.
			messages = append(messages,
				provider.ChatMessage{Role: "assistant", Content: raw},
				provider.ChatMessage{Role: "user", Content: jsonRepairHint},
			)
			continue
		}
		files, err := parseFiles(data, targetPath)
		if err != nil {
			dump := g.dumpAIFailure(practice.PracticeID, attempt, raw, err)
			if dump != "" {
				dumpPaths = append(dumpPaths, dump)
			}
			lastErr = err
			messages = append(messages,
				provider.ChatMessage{Role: "assistant", Content: raw},
				provider.ChatMessage{Role: "user", Content: jsonRepairHint},
			)
			continue
		}
		if err := ValidatePaths(files, g.Settings.PathAllowlist); err != nil {
			lastErr = err
			continue
		}
		files, err = nav.ApplyToFiles(files, nav.ApplyOptions{
			CRepoRoot: cRepoRoot,
			Service:   doc.Service,
			Slug:      doc.Slug,
		})
		if err != nil {
			lastErr = err
			continue
		}
		if err := requireBilingualBodies(files, g.Settings.CDocsRoot, doc.Service, doc.Slug); err != nil {
			lastErr = err
			continue
		}
		summary, _ := data["summary"].(string)
		return &model.GenerateResult{
			PracticeID:  practice.PracticeID,
			Files:       files,
			Summary:     summary,
			RawResponse: raw,
		}, nil
	}
	if len(dumpPaths) > 0 {
		return nil, fmt.Errorf("doc generation failed for %s: %w (see %s)", practice.PracticeID, lastErr, dumpPaths[len(dumpPaths)-1])
	}
	return nil, fmt.Errorf("doc generation failed for %s: %w", practice.PracticeID, lastErr)
}

const jsonRepairHint = `Previous output was not valid/complete JSON. Reply with ONE JSON object only — no markdown fences, no commentary.
Ensure every string (especially files[].content) has correct escaping, and that all braces/brackets are closed. Keep bilingual bodies but prefer shorter HCL excerpts if needed to avoid truncation.`

func (g *DocGenerator) dumpAIFailure(practiceID string, attempt int, raw string, cause error) string {
	if g == nil || g.Settings == nil {
		return ""
	}
	slug := strings.ReplaceAll(practiceID, "/", "__")
	dir := filepath.Join(g.Settings.AbsoluteWorkDir(), "ai-failures", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	base := fmt.Sprintf("attempt-%d", attempt+1)
	rawPath := filepath.Join(dir, base+".raw.txt")
	errPath := filepath.Join(dir, base+".err.txt")
	_ = os.WriteFile(rawPath, []byte(raw), 0o644)
	_ = os.WriteFile(errPath, []byte(fmt.Sprintf("practice_id=%s\nattempt=%d\nbytes=%d\nerror=%v\n", practiceID, attempt+1, len(raw), cause)), 0o644)
	return rawPath
}

func parseFiles(data map[string]any, fallbackPath string) ([]model.DocFileChange, error) {
	rawFiles, ok := data["files"].([]any)
	if ok && len(rawFiles) > 0 {
		var files []model.DocFileChange
		for _, item := range rawFiles {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			path, _ := m["path"].(string)
			content, _ := m["content"].(string)
			action, _ := m["action"].(string)
			if action == "" {
				action = "create"
			}
			if path == "" || content == "" {
				continue
			}
			files = append(files, model.DocFileChange{Path: filepath.ToSlash(path), Content: content, Action: action})
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("AI JSON missing usable files[]")
		}
		return files, nil
	}
	if content, ok := data["content"].(string); ok && content != "" {
		return []model.DocFileChange{{Path: fallbackPath, Content: content, Action: "create"}}, nil
	}
	return nil, fmt.Errorf("AI JSON missing files[]")
}

func requireBilingualBodies(files []model.DocFileChange, _docsRoot, service, slug string) error {
	zh := filepath.ToSlash(filepath.Join("docs/zh-cn/best-practices", service, slug+".md"))
	en := filepath.ToSlash(filepath.Join("docs/en-us/best-practices", service, slug+".md"))
	var hasZh, hasEn bool
	for _, f := range files {
		switch filepath.ToSlash(f.Path) {
		case zh:
			hasZh = true
		case en:
			hasEn = true
		}
	}
	if !hasZh || !hasEn {
		return fmt.Errorf("bilingual bodies required: missing %v", map[string]bool{"zh": hasZh, "en": hasEn})
	}
	return nil
}
