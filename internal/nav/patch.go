package nav

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	summaryServiceRe  = regexp.MustCompile(`^\s{2}\* \[([^\]]+)\]\(best-practices/([^)/]+)/\)\s*$`)
	summaryPracticeRe = regexp.MustCompile(`^\s{4}\* \[([^\]]+)\]\(best-practices/([^)/]+)/([^)]+\.md)\)\s*$`)
	indexListRe       = regexp.MustCompile(`^\* \[([^\]]+)\]\(([^)]+\.md)\)\s*-\s*(.+)$`)
	readmeNavRe       = regexp.MustCompile(`(?m)^### \[([^\]]+)\]\(([^)/]+)/index\.md\)\s*$`)
)

// LocaleStrings holds language-specific navigation labels.
type LocaleStrings struct {
	IntroLabel       string // SUMMARY intro link text
	IndexListHeading string // e.g. "## 最佳实践列表"
	IndexListLead    string // line under heading before items
	ReadmeNavHeading string // e.g. "## 文档导航"
	SentenceEnd      string // "。" or "."
}

// ZhCN is Chinese documentation locale strings.
var ZhCN = LocaleStrings{
	IntroLabel:       "简介",
	IndexListHeading: "## 最佳实践列表",
	IndexListLead:    "本章节包含以下最佳实践：",
	ReadmeNavHeading: "## 文档导航",
	SentenceEnd:      "。",
}

// EnUS is English documentation locale strings.
var EnUS = LocaleStrings{
	IntroLabel:       "Introduction",
	IndexListHeading: "## Best Practices List",
	IndexListLead:    "This section contains the following best practices:",
	ReadmeNavHeading: "## Documentation Navigation",
	SentenceEnd:      ".",
}

// PatchSUMMARY inserts a practice link under the service section (surgical).
// Practices under a service are ordered by practiceTitleAsc (typically the English H1/title).
func PatchSUMMARY(content, service, serviceLabel, practiceSlug, practiceTitle, introLabel string) (string, error) {
	return patchSUMMARY(content, service, serviceLabel, practiceSlug, practiceTitle, introLabel, nil)
}

// PatchSUMMARYFollowOrder inserts like PatchSUMMARY but orders practices by orderedFiles
// (e.g. English SUMMARY practice file order) instead of by title.
func PatchSUMMARYFollowOrder(content, service, serviceLabel, practiceSlug, practiceTitle, introLabel string, orderedFiles []string) (string, error) {
	return patchSUMMARY(content, service, serviceLabel, practiceSlug, practiceTitle, introLabel, orderedFiles)
}

func patchSUMMARY(content, service, serviceLabel, practiceSlug, practiceTitle, introLabel string, orderedFiles []string) (string, error) {
	service = strings.TrimSpace(service)
	practiceSlug = strings.TrimSuffix(strings.TrimSpace(practiceSlug), ".md")
	if service == "" || practiceSlug == "" {
		return "", fmt.Errorf("service and practice slug are required")
	}
	if serviceLabel == "" {
		serviceLabel = strings.ToUpper(service)
	}
	if practiceTitle == "" {
		practiceTitle = practiceSlug
	}
	if introLabel == "" {
		introLabel = "简介"
	}

	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	practicePath := fmt.Sprintf("best-practices/%s/%s.md", service, practiceSlug)
	practiceFile := practiceSlug + ".md"
	practiceLine := fmt.Sprintf("    * [%s](%s)", practiceTitle, practicePath)
	introLine := fmt.Sprintf("    * [%s](best-practices/%s/index.md)", introLabel, service)
	serviceLine := fmt.Sprintf("  * [%s](best-practices/%s/)", serviceLabel, service)

	for _, line := range lines {
		if strings.Contains(line, "("+practicePath+")") {
			return ensureTrailingNewline(strings.Join(lines, "\n")), nil
		}
	}

	svcStart, svcEnd, found := findServiceBlock(lines, service)
	if found {
		type prac struct {
			file  string
			title string
			line  string
		}
		var prefix []string // intro / non-practice lines kept in place before practices
		var practices []prac
		prefix = append(prefix, lines[svcStart])
		for i := svcStart + 1; i < svcEnd; i++ {
			line := lines[i]
			if m := summaryPracticeRe.FindStringSubmatch(line); m != nil {
				practices = append(practices, prac{file: m[3], title: m[1], line: line})
				continue
			}
			if len(practices) == 0 {
				prefix = append(prefix, line)
			}
			// trailing non-practice after practices is dropped/ignored (none expected)
		}
		practices = append(practices, prac{file: practiceFile, title: practiceTitle, line: practiceLine})
		if len(orderedFiles) > 0 {
			rank := map[string]int{}
			for i, f := range orderedFiles {
				rank[f] = i
			}
			sort.SliceStable(practices, func(i, j int) bool {
				ri, okI := rank[practices[i].file]
				rj, okJ := rank[practices[j].file]
				if okI && okJ {
					return ri < rj
				}
				if okI != okJ {
					return okI
				}
				return strings.ToLower(practices[i].title) < strings.ToLower(practices[j].title)
			})
		} else {
			sort.SliceStable(practices, func(i, j int) bool {
				return strings.ToLower(practices[i].title) < strings.ToLower(practices[j].title)
			})
		}
		out := make([]string, 0, len(lines)+1)
		out = append(out, lines[:svcStart]...)
		out = append(out, prefix...)
		for _, p := range practices {
			out = append(out, p.line)
		}
		out = append(out, lines[svcEnd:]...)
		return ensureTrailingNewline(strings.Join(out, "\n")), nil
	}

	block := []string{serviceLine, introLine, practiceLine}
	insertAt := newServiceInsertIndex(lines, service)
	out := make([]string, 0, len(lines)+len(block))
	out = append(out, lines[:insertAt]...)
	out = append(out, block...)
	out = append(out, lines[insertAt:]...)
	return ensureTrailingNewline(strings.Join(out, "\n")), nil
}

// PracticeFilesFromSUMMARY returns practice filenames (e.g. redis_account.md) under a service, in listed order.
func PracticeFilesFromSUMMARY(content, service string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	start, end, ok := findServiceBlock(lines, service)
	if !ok {
		return nil
	}
	var files []string
	for i := start + 1; i < end; i++ {
		if m := summaryPracticeRe.FindStringSubmatch(lines[i]); m != nil {
			files = append(files, m[3])
		}
	}
	return files
}

func findServiceBlock(lines []string, service string) (start, end int, found bool) {
	marker := fmt.Sprintf("(best-practices/%s/)", service)
	start = -1
	for i, line := range lines {
		if m := summaryServiceRe.FindStringSubmatch(line); m != nil && m[2] == service {
			start = i
			break
		}
		if strings.HasPrefix(line, "  * ") && strings.Contains(line, marker) {
			start = i
			break
		}
	}
	if start < 0 {
		return 0, 0, false
	}
	end = len(lines)
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		if summaryServiceRe.MatchString(line) {
			end = i
			break
		}
		if strings.HasPrefix(line, "* ") {
			end = i
			break
		}
		if strings.HasPrefix(line, "  * ") && !strings.HasPrefix(line, "    * ") {
			end = i
			break
		}
	}
	return start, end, true
}

func newServiceInsertIndex(lines []string, service string) int {
	bestIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "](best-practices/)") || strings.Contains(line, "](best-practices)") {
			bestIdx = i
			break
		}
	}
	if bestIdx < 0 {
		return len(lines)
	}
	insertAt := bestIdx + 1
	for i := bestIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "* ") {
			return i
		}
		m := summaryServiceRe.FindStringSubmatch(line)
		if m == nil {
			if strings.HasPrefix(line, "  * ") {
				insertAt = i + 1
			}
			continue
		}
		if m[2] > service {
			return i
		}
		_, end, ok := findServiceBlock(lines, m[2])
		if ok {
			insertAt = end
			i = end - 1
		} else {
			insertAt = i + 1
		}
	}
	return insertAt
}

// PatchServiceIndex inserts one list item under the locale list heading, sorted by practice title.
func PatchServiceIndex(content, practiceSlug, practiceTitle, oneLiner string, loc LocaleStrings) (string, error) {
	return patchServiceIndex(content, practiceSlug, practiceTitle, oneLiner, loc, nil)
}

// PatchServiceIndexFollowOrder inserts like PatchServiceIndex but orders list items by orderedFiles.
func PatchServiceIndexFollowOrder(content, practiceSlug, practiceTitle, oneLiner string, loc LocaleStrings, orderedFiles []string) (string, error) {
	return patchServiceIndex(content, practiceSlug, practiceTitle, oneLiner, loc, orderedFiles)
}

func patchServiceIndex(content, practiceSlug, practiceTitle, oneLiner string, loc LocaleStrings, orderedFiles []string) (string, error) {
	practiceSlug = strings.TrimSuffix(strings.TrimSpace(practiceSlug), ".md")
	if practiceSlug == "" {
		return "", fmt.Errorf("practice slug required")
	}
	if practiceTitle == "" {
		practiceTitle = practiceSlug
	}
	if oneLiner == "" {
		oneLiner = practiceTitle
	}
	end := loc.SentenceEnd
	if end == "" {
		end = "."
	}
	oneLiner = strings.TrimRight(strings.TrimSpace(oneLiner), "。.") + end

	heading := loc.IndexListHeading
	if heading == "" {
		heading = "## Best Practices List"
	}
	lead := loc.IndexListLead

	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	link := practiceSlug + ".md"
	item := fmt.Sprintf("* [%s](%s) - %s", practiceTitle, link, oneLiner)

	if strings.Contains(content, "]("+link+")") {
		return content, nil
	}

	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	listStart := -1
	wantHeading := strings.TrimSpace(heading)
	for i, line := range lines {
		if strings.TrimSpace(line) == wantHeading {
			listStart = i
			break
		}
	}
	if listStart < 0 {
		extra := []string{"", heading, ""}
		if lead != "" {
			extra = append(extra, lead, "")
		}
		extra = append(extra, item)
		lines = append(lines, extra...)
		return ensureTrailingNewline(strings.Join(lines, "\n")), nil
	}

	listEnd := len(lines)
	for i := listStart + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			listEnd = i
			break
		}
	}

	type entry struct {
		file  string
		title string
		line  string
	}
	var entries []entry
	firstItem, lastItem := -1, -1
	for i := listStart + 1; i < listEnd; i++ {
		m := indexListRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		if firstItem < 0 {
			firstItem = i
		}
		lastItem = i
		entries = append(entries, entry{file: m[2], title: m[1], line: lines[i]})
	}

	entries = append(entries, entry{file: link, title: practiceTitle, line: item})
	if len(orderedFiles) > 0 {
		rank := map[string]int{}
		for i, f := range orderedFiles {
			rank[f] = i
		}
		sort.SliceStable(entries, func(i, j int) bool {
			ri, okI := rank[entries[i].file]
			rj, okJ := rank[entries[j].file]
			if okI && okJ {
				return ri < rj
			}
			if okI != okJ {
				return okI
			}
			return strings.ToLower(entries[i].title) < strings.ToLower(entries[j].title)
		})
	} else {
		sort.SliceStable(entries, func(i, j int) bool {
			return strings.ToLower(entries[i].title) < strings.ToLower(entries[j].title)
		})
	}

	newLines := make([]string, 0, len(entries))
	for _, e := range entries {
		newLines = append(newLines, e.line)
	}

	out := make([]string, 0, len(lines)+1)
	if firstItem < 0 {
		out = append(out, lines[:listEnd]...)
		if listEnd > 0 && lines[listEnd-1] != "" {
			out = append(out, "")
		}
		out = append(out, newLines...)
		out = append(out, lines[listEnd:]...)
	} else {
		out = append(out, lines[:firstItem]...)
		out = append(out, newLines...)
		out = append(out, lines[lastItem+1:]...)
	}
	return ensureTrailingNewline(strings.Join(out, "\n")), nil
}

// PracticeFilesFromIndex returns practice filenames from a service index list, in listed order.
func PracticeFilesFromIndex(content string, loc LocaleStrings) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	heading := loc.IndexListHeading
	if heading == "" {
		heading = "## Best Practices List"
	}
	listStart := -1
	want := strings.TrimSpace(heading)
	for i, line := range lines {
		if strings.TrimSpace(line) == want {
			listStart = i
			break
		}
	}
	if listStart < 0 {
		return nil
	}
	var files []string
	for i := listStart + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			break
		}
		if m := indexListRe.FindStringSubmatch(lines[i]); m != nil {
			files = append(files, m[2])
		}
	}
	return files
}

// PatchBestPracticesREADME inserts a service navigation block sorted by service path.
func PatchBestPracticesREADME(content, service, heading, blurb string, loc LocaleStrings) (string, error) {
	service = strings.TrimSpace(service)
	if service == "" {
		return "", fmt.Errorf("service required")
	}
	if heading == "" {
		heading = fmt.Sprintf("%s Best Practices", strings.ToUpper(service))
	}
	if strings.TrimSpace(blurb) == "" {
		return "", fmt.Errorf("readme blurb empty for service %s; refuse placeholder", service)
	}

	navHeading := loc.ReadmeNavHeading
	if navHeading == "" {
		navHeading = "## Documentation Navigation"
	}

	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	link := service + "/index.md"
	block := fmt.Sprintf("### [%s](%s)\n\n%s\n", heading, link, strings.TrimSpace(blurb))

	navIdx := strings.Index(content, navHeading)
	if navIdx < 0 {
		// try the other locale heading as fallback
		for _, alt := range []string{"## Documentation Navigation", "## 文档导航"} {
			if i := strings.Index(content, alt); i >= 0 {
				navIdx = i
				navHeading = alt
				break
			}
		}
	}
	if navIdx < 0 {
		return ensureTrailingNewline(content + "\n" + navHeading + "\n\n" + block), nil
	}

	afterNavHeader := content[navIdx:]
	headerEnd := strings.Index(afterNavHeader, "\n")
	if headerEnd < 0 {
		return ensureTrailingNewline(content + "\n\n" + block), nil
	}
	prefix := content[:navIdx+headerEnd+1]
	rest := afterNavHeader[headerEnd+1:]

	nextH2 := regexp.MustCompile(`(?m)^## `).FindStringIndex(rest)
	navBody, suffix := rest, ""
	if nextH2 != nil {
		navBody = rest[:nextH2[0]]
		suffix = rest[nextH2[0]:]
	}

	type sec struct {
		key  string
		text string
	}
	var sections []sec
	parts := readmeNavRe.FindAllStringIndex(navBody, -1)
	if len(parts) == 0 {
		return ensureTrailingNewline(prefix + "\n" + block + "\n" + suffix), nil
	}
	replaced := false
	for i, locIdx := range parts {
		end := len(navBody)
		if i+1 < len(parts) {
			end = parts[i+1][0]
		}
		chunk := strings.TrimRight(navBody[locIdx[0]:end], "\n") + "\n"
		m := readmeNavRe.FindStringSubmatch(navBody[locIdx[0]:locIdx[1]])
		key := ""
		if m != nil {
			key = m[2]
		}
		if key == service {
			sections = append(sections, sec{key: service, text: block})
			replaced = true
			continue
		}
		sections = append(sections, sec{key: key, text: chunk})
	}
	if !replaced {
		sections = append(sections, sec{key: service, text: block})
	}
	sort.SliceStable(sections, func(i, j int) bool { return sections[i].key < sections[j].key })

	var b strings.Builder
	b.WriteString(prefix)
	if !strings.HasSuffix(prefix, "\n") {
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	for _, s := range sections {
		b.WriteString(strings.TrimRight(s.text, "\n"))
		b.WriteString("\n\n")
	}
	b.WriteString(suffix)
	return ensureTrailingNewline(b.String()), nil
}

// IsDestructiveUpdate reports whether updated content likely dropped too much of baseline.
func IsDestructiveUpdate(baseline, updated string) bool {
	baseLines := nonEmptyLines(baseline)
	upLines := nonEmptyLines(updated)
	if len(baseLines) == 0 {
		return false
	}
	if len(upLines) < len(baseLines)*8/10 {
		return true
	}
	if strings.Contains(baseline, "# Summary") && !strings.Contains(updated, "# Summary") {
		return true
	}
	if strings.Contains(baseline, "best-practices/anti-ddos/") && !strings.Contains(updated, "best-practices/anti-ddos/") {
		return true
	}
	return false
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func ensureTrailingNewline(s string) string {
	if !strings.HasSuffix(s, "\n") {
		return s + "\n"
	}
	return s
}

// ServiceLabelFromSUMMARY finds existing display label for a service, if any.
func ServiceLabelFromSUMMARY(content, service string) string {
	for _, line := range strings.Split(content, "\n") {
		m := summaryServiceRe.FindStringSubmatch(line)
		if m != nil && m[2] == service {
			return m[1]
		}
	}
	return ""
}
