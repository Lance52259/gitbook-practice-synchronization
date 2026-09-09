package nav

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/chnsz/gitbook-practice-synchronization/internal/model"
)

var (
	indexItemOneLinerRe = regexp.MustCompile(`(?m)^\*\s+\[[^\]]+\]\(([^)]+\.md)\)\s+-\s+(.+)$`)
	resourceBulletRe    = regexp.MustCompile(`(?m)^[\*\-]\s*\[([^\]]+)\]\([^)]*\)`)
	resourceTypeRe      = regexp.MustCompile(`(?i)[（(]huaweicloud_([a-z0-9_]+)[）)]`)
	huaweicloudTokenRe  = regexp.MustCompile(`(?i)huaweicloud_[a-z0-9_]+`)
)

// DefaultEnglishOneLiner builds an index-list blurb in the valuable C-repo style:
//
//	Introduces how to use Terraform to automatically {lowerFirst(title)}[, including …].
func DefaultEnglishOneLiner(title string, including []string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "the best practice"
	}
	core := "Introduces how to use Terraform to automatically " + LowerFirst(title)
	if phrase := joinIncludingEN(including); phrase != "" {
		return core + ", including " + phrase + "."
	}
	return ensureSentenceEnd(core, ".")
}

// DefaultChineseOneLiner builds the ZH counterpart:
//
//	介绍如何使用Terraform自动化{title}[，包括…]。
func DefaultChineseOneLiner(title string, including []string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "该最佳实践"
	}
	core := "介绍如何使用Terraform自动化" + title
	if phrase := joinIncludingZH(including); phrase != "" {
		return core + "，包括" + phrase + "。"
	}
	return ensureSentenceEnd(core, "。")
}

// LowerFirst lowercases the first letter of s (Unicode-aware).
func LowerFirst(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError && size == 1 {
		return s
	}
	return string(unicode.ToLower(r)) + s[size:]
}

// IsWeakOneLiner reports placeholder blurbs (quoted title / "automate Title" without including /
// or raw Terraform resource type tokens that existing C-repo index lines never show).
func IsWeakOneLiner(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if strings.ContainsAny(s, "«»《》「」") {
		return true
	}
	if huaweicloudTokenRe.MatchString(s) {
		return true
	}
	lower := strings.ToLower(s)
	hasIncluding := strings.Contains(lower, "including") || strings.Contains(s, "包括")
	if strings.Contains(lower, "to automate ") && !hasIncluding {
		return true
	}
	if strings.Contains(s, "自动化完成") && !hasIncluding {
		return true
	}
	return false
}

// OneLinerFromIndexList returns the description after " - " for link file (e.g. foo.md).
func OneLinerFromIndexList(indexContent, file string) string {
	file = filepath.Base(strings.TrimSpace(file))
	if file == "" {
		return ""
	}
	indexContent = strings.ReplaceAll(indexContent, "\r\n", "\n")
	for _, m := range indexItemOneLinerRe.FindAllStringSubmatch(indexContent, -1) {
		if filepath.Base(m[1]) != file {
			continue
		}
		desc := strings.TrimSpace(m[2])
		desc = strings.TrimRight(desc, " \t")
		return desc
	}
	return ""
}

// ResourceHintsFromPractice extracts short "including …" phrases from a practice body
// (Related Resources / 相关资源 link labels and common resource-type categories).
func ResourceHintsFromPractice(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	section := relatedResourcesSection(content)
	if section == "" {
		section = content
	}

	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		s = collapseSpace(strings.TrimSpace(s))
		if s == "" {
			return
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}

	for _, m := range resourceBulletRe.FindAllStringSubmatch(section, -1) {
		label := m[1]
		if cat := categoryFromResourceLabel(label); cat != "" {
			add(cat)
			continue
		}
		if cleaned := cleanResourceDisplayLabel(label); cleaned != "" {
			add(cleaned)
		}
	}

	// Cap length so the index line stays readable.
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

// cleanResourceDisplayLabel strips Terraform type suffixes such as
// (huaweicloud_dcs_custom_template) / （huaweicloud_dcs_custom_template）.
// Existing C-repo ZH index lines never expose raw resource type names.
func cleanResourceDisplayLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	label = resourceTypeRe.ReplaceAllString(label, "")
	label = huaweicloudTokenRe.ReplaceAllString(label, "")
	label = strings.TrimSpace(label)
	label = strings.TrimRight(label, "（( ")
	label = strings.TrimSpace(label)
	if huaweicloudTokenRe.MatchString(label) {
		return ""
	}
	return label
}

func relatedResourcesSection(content string) string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "## ") {
			continue
		}
		title := strings.TrimSpace(strings.TrimPrefix(trim, "## "))
		lower := strings.ToLower(title)
		if strings.Contains(lower, "related resource") || strings.HasPrefix(title, "相关资源") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trim, "## ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func categoryFromResourceLabel(label string) string {
	m := resourceTypeRe.FindStringSubmatch(label)
	if m == nil {
		return ""
	}
	typ := strings.ToLower(m[1])
	// Drop leading service token when present (dcs_instance → instance).
	switch {
	case typ == "vpc" || (strings.HasSuffix(typ, "_vpc") && !strings.Contains(typ, "subnet")):
		return "VPC creation"
	case strings.Contains(typ, "subnet"):
		return "subnet configuration"
	case strings.Contains(typ, "secgroup") || strings.Contains(typ, "security_group"):
		return "security group configuration"
	case strings.Contains(typ, "whitelist"):
		return "whitelist management"
	case strings.Contains(typ, "backup"):
		return "backup policy"
	case strings.HasSuffix(typ, "_instance") || strings.Contains(typ, "instance"):
		return "instance configuration"
	case strings.Contains(typ, "network"):
		return "basic network setup"
	default:
		return ""
	}
}

func joinIncludingEN(parts []string) string {
	parts = compactNonEmpty(parts)
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
	}
}

func joinIncludingZH(parts []string) string {
	parts = compactNonEmpty(parts)
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + "和" + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], "、") + "和" + parts[len(parts)-1]
	}
}

func compactNonEmpty(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func ensureSentenceEnd(s, end string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "。.")
	if s == "" {
		return s
	}
	return s + end
}

// BuildPracticeOneLiners prefers explicit options, then a non-weak AI index blurb,
// then a body-derived default with resource hints.
func BuildPracticeOneLiners(opt ApplyOptions, files []model.DocFileChange, aiZhIndex, aiEnIndex string) (zhOne, enOne string) {
	zhTitle := opt.ZhTitle
	enTitle := opt.EnTitle
	if zhTitle == "" {
		zhTitle = TitleFromFiles(files, zhDocsRoot, opt.Service, opt.Slug)
	}
	if enTitle == "" {
		enTitle = TitleFromFiles(files, enDocsRoot, opt.Service, opt.Slug)
	}
	if enTitle == opt.Slug && zhTitle != opt.Slug {
		enTitle = zhTitle
	}

	link := opt.Slug + ".md"
	enBody := contentFromFiles(files, filepath.ToSlash(filepath.Join(enDocsRoot, opt.Service, link)))
	zhBody := contentFromFiles(files, filepath.ToSlash(filepath.Join(zhDocsRoot, opt.Service, link)))
	enHints := sanitizeIncludingHints(ResourceHintsFromPractice(enBody))
	zhHints := sanitizeIncludingHints(ResourceHintsFromPractice(zhBody))
	if len(zhHints) == 0 {
		zhHints = translateHintsToZH(enHints)
	} else {
		// categoryFromResourceLabel yields English phrases even from ZH bodies.
		zhHints = translateHintsToZH(zhHints)
	}

	enOne = strings.TrimSpace(opt.EnOneLiner)
	if enOne == "" {
		if ol := OneLinerFromIndexList(aiEnIndex, link); ol != "" && !IsWeakOneLiner(ol) {
			enOne = ol
		} else {
			enOne = DefaultEnglishOneLiner(enTitle, enHints)
		}
	}
	zhOne = strings.TrimSpace(opt.ZhOneLiner)
	if zhOne == "" {
		if ol := OneLinerFromIndexList(aiZhIndex, link); ol != "" && !IsWeakOneLiner(ol) {
			zhOne = ol
		} else {
			zhOne = DefaultChineseOneLiner(zhTitle, zhHints)
		}
	}
	return zhOne, enOne
}

func sanitizeIncludingHints(hints []string) []string {
	out := make([]string, 0, len(hints))
	seen := map[string]struct{}{}
	for _, h := range hints {
		h = cleanResourceDisplayLabel(h)
		h = collapseSpace(strings.TrimSpace(h))
		if h == "" || huaweicloudTokenRe.MatchString(h) {
			continue
		}
		key := strings.ToLower(h)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, h)
	}
	return out
}

func contentFromFiles(files []model.DocFileChange, want string) string {
	want = filepath.ToSlash(want)
	for _, f := range files {
		if filepath.ToSlash(f.Path) == want {
			return f.Content
		}
	}
	return ""
}

func translateHintsToZH(en []string) []string {
	repl := map[string]string{
		"vpc creation":                  "VPC创建",
		"subnet configuration":          "子网配置",
		"security group configuration":  "安全组配置",
		"whitelist management":          "白名单管理",
		"backup policy":                 "备份策略",
		"instance configuration":        "实例配置",
		"basic network setup":           "基础网络设置",
	}
	out := make([]string, 0, len(en))
	for _, h := range en {
		if z, ok := repl[strings.ToLower(h)]; ok {
			out = append(out, z)
		} else {
			out = append(out, h)
		}
	}
	return out
}
