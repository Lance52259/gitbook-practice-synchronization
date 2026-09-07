package nav

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Lance52259/gitbook-practice-synchronization/internal/model"
)

var h1Re = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)

const (
	zhDocsRoot = "docs/zh-cn/best-practices"
	enDocsRoot = "docs/en-us/best-practices"
)

// ApplyOptions controls bilingual surgical navigation updates (Skill steps 3–8).
type ApplyOptions struct {
	CRepoRoot string
	Service   string
	Slug      string

	ServiceLabel string

	ZhTitle   string
	EnTitle   string
	ZhOneLiner string
	EnOneLiner string

	ZhREADMEHeading string
	EnREADMEHeading string
	ZhREADMEBlurb   string
	EnREADMEBlurb   string
}

// ApplyToFiles enforces Skill order: EN nav first (alpha), then ZH nav following same paths.
func ApplyToFiles(files []model.DocFileChange, opt ApplyOptions) ([]model.DocFileChange, error) {
	if opt.CRepoRoot == "" || opt.Service == "" || opt.Slug == "" {
		return files, nil
	}

	zhTitle := opt.ZhTitle
	if zhTitle == "" {
		zhTitle = TitleFromFiles(files, zhDocsRoot, opt.Service, opt.Slug)
	}
	enTitle := opt.EnTitle
	if enTitle == "" {
		enTitle = TitleFromFiles(files, enDocsRoot, opt.Service, opt.Slug)
	}
	if enTitle == opt.Slug && zhTitle != opt.Slug {
		// weak fallback
		enTitle = zhTitle
	}
	zhOne := opt.ZhOneLiner
	if zhOne == "" {
		zhOne = fmt.Sprintf("介绍如何使用Terraform自动化完成「%s」", zhTitle)
	}
	enOne := opt.EnOneLiner
	if enOne == "" {
		enOne = fmt.Sprintf("Introduces how to use Terraform to automate «%s»", enTitle)
	}

	label := opt.ServiceLabel
	if label == "" {
		label = strings.ToUpper(opt.Service)
	}

	zhSummaryPath := "docs/zh-cn/SUMMARY.md"
	enSummaryPath := "docs/en-us/SUMMARY.md"
	zhIndexPath := filepath.ToSlash(filepath.Join(zhDocsRoot, opt.Service, "index.md"))
	enIndexPath := filepath.ToSlash(filepath.Join(enDocsRoot, opt.Service, "index.md"))
	zhReadmePath := filepath.ToSlash(filepath.Join(zhDocsRoot, "README.md"))
	enReadmePath := filepath.ToSlash(filepath.Join(enDocsRoot, "README.md"))

	zhSummary, zhSummaryOK := readFile(opt.CRepoRoot, zhSummaryPath)
	enSummary, enSummaryOK := readFile(opt.CRepoRoot, enSummaryPath)
	if !zhSummaryOK || strings.TrimSpace(zhSummary) == "" {
		return nil, fmt.Errorf("SUMMARY.md must exist at %s", zhSummaryPath)
	}
	if !enSummaryOK || strings.TrimSpace(enSummary) == "" {
		return nil, fmt.Errorf("SUMMARY.md must exist at %s", enSummaryPath)
	}
	if existing := ServiceLabelFromSUMMARY(enSummary, opt.Service); existing != "" {
		label = existing
	} else if existing := ServiceLabelFromSUMMARY(zhSummary, opt.Service); existing != "" {
		label = existing
	}

	zhIndexBase, zhIndexExists := readFile(opt.CRepoRoot, zhIndexPath)
	enIndexBase, enIndexExists := readFile(opt.CRepoRoot, enIndexPath)
	zhReadmeBase, zhReadmeOK := readFile(opt.CRepoRoot, zhReadmePath)
	enReadmeBase, enReadmeOK := readFile(opt.CRepoRoot, enReadmePath)

	zhDir := filepath.Join(opt.CRepoRoot, filepath.FromSlash(zhDocsRoot), opt.Service)
	enDir := filepath.Join(opt.CRepoRoot, filepath.FromSlash(enDocsRoot), opt.Service)
	newService := (!dirExists(zhDir) && !zhIndexExists) || (!dirExists(enDir) && !enIndexExists)

	navPaths := map[string]struct{}{
		zhSummaryPath: {}, enSummaryPath: {},
		zhIndexPath: {}, enIndexPath: {},
		zhReadmePath: {}, enReadmePath: {},
	}

	out := make([]model.DocFileChange, 0, len(files)+8)
	var aiZhIndex, aiEnIndex *model.DocFileChange
	for _, f := range files {
		p := filepath.ToSlash(f.Path)
		if _, isNav := navPaths[p]; isNav {
			switch p {
			case zhIndexPath:
				if newService {
					cp := f
					aiZhIndex = &cp
				}
			case enIndexPath:
				if newService {
					cp := f
					aiEnIndex = &cp
				}
			}
			continue
		}
		out = append(out, f)
	}

	// --- Steps 3–5: English nav (defines alphabetical order) ---
	enSummaryPatched, err := PatchSUMMARY(enSummary, opt.Service, label, opt.Slug, enTitle, EnUS.IntroLabel)
	if err != nil {
		return nil, fmt.Errorf("patch en SUMMARY: %w", err)
	}
	if IsDestructiveUpdate(enSummary, enSummaryPatched) {
		return nil, fmt.Errorf("refusing destructive en SUMMARY patch")
	}
	out = append(out, model.DocFileChange{Path: enSummaryPath, Action: "update", Content: enSummaryPatched})

	if newService {
		if aiEnIndex == nil || strings.TrimSpace(aiEnIndex.Content) == "" {
			return nil, fmt.Errorf("new service %s requires create of %s", opt.Service, enIndexPath)
		}
		enIdx := aiEnIndex.Content
		if patched, err := PatchServiceIndex(enIdx, opt.Slug, enTitle, enOne, EnUS); err == nil {
			enIdx = patched
		}
		out = append(out, model.DocFileChange{Path: enIndexPath, Action: "create", Content: ensureTrailingNewline(enIdx)})
		if enReadmeOK {
			heading, blurb := ReadmeNavFromIndex(enIdx, EnUS, label)
			if opt.EnREADMEHeading != "" {
				heading = opt.EnREADMEHeading
			}
			if opt.EnREADMEBlurb != "" {
				blurb = opt.EnREADMEBlurb
			}
			if strings.TrimSpace(blurb) == "" {
				return nil, fmt.Errorf("cannot excerpt EN README blurb from %s; index must include a ## What is … section with intro paragraph", enIndexPath)
			}
			patched, err := PatchBestPracticesREADME(enReadmeBase, opt.Service, heading, blurb, EnUS)
			if err != nil {
				return nil, fmt.Errorf("patch en README: %w", err)
			}
			if IsDestructiveUpdate(enReadmeBase, patched) {
				return nil, fmt.Errorf("refusing destructive en README patch")
			}
			out = append(out, model.DocFileChange{Path: enReadmePath, Action: "update", Content: patched})
		}
	} else {
		if !enIndexExists {
			return nil, fmt.Errorf("existing service missing %s", enIndexPath)
		}
		patched, err := PatchServiceIndex(enIndexBase, opt.Slug, enTitle, enOne, EnUS)
		if err != nil {
			return nil, fmt.Errorf("patch en index: %w", err)
		}
		if IsDestructiveUpdate(enIndexBase, patched) {
			return nil, fmt.Errorf("refusing destructive en index patch")
		}
		out = append(out, model.DocFileChange{Path: enIndexPath, Action: "update", Content: patched})
	}

	// --- Steps 6–8: Chinese nav (follow English path order) ---
	zhSummaryPatched, err := PatchSUMMARY(zhSummary, opt.Service, label, opt.Slug, zhTitle, ZhCN.IntroLabel)
	if err != nil {
		return nil, fmt.Errorf("patch zh SUMMARY: %w", err)
	}
	if IsDestructiveUpdate(zhSummary, zhSummaryPatched) {
		return nil, fmt.Errorf("refusing destructive zh SUMMARY patch")
	}
	out = append(out, model.DocFileChange{Path: zhSummaryPath, Action: "update", Content: zhSummaryPatched})

	if newService {
		if aiZhIndex == nil || strings.TrimSpace(aiZhIndex.Content) == "" {
			return nil, fmt.Errorf("new service %s requires create of %s", opt.Service, zhIndexPath)
		}
		zhIdx := aiZhIndex.Content
		if patched, err := PatchServiceIndex(zhIdx, opt.Slug, zhTitle, zhOne, ZhCN); err == nil {
			zhIdx = patched
		}
		out = append(out, model.DocFileChange{Path: zhIndexPath, Action: "create", Content: ensureTrailingNewline(zhIdx)})
		if zhReadmeOK {
			heading, blurb := ReadmeNavFromIndex(zhIdx, ZhCN, label)
			if opt.ZhREADMEHeading != "" {
				heading = opt.ZhREADMEHeading
			}
			if opt.ZhREADMEBlurb != "" {
				blurb = opt.ZhREADMEBlurb
			}
			if strings.TrimSpace(blurb) == "" {
				return nil, fmt.Errorf("cannot excerpt ZH README blurb from %s; index must include a ## 什么是… section with intro paragraph", zhIndexPath)
			}
			patched, err := PatchBestPracticesREADME(zhReadmeBase, opt.Service, heading, blurb, ZhCN)
			if err != nil {
				return nil, fmt.Errorf("patch zh README: %w", err)
			}
			if IsDestructiveUpdate(zhReadmeBase, patched) {
				return nil, fmt.Errorf("refusing destructive zh README patch")
			}
			out = append(out, model.DocFileChange{Path: zhReadmePath, Action: "update", Content: patched})
		}
	} else {
		if !zhIndexExists {
			return nil, fmt.Errorf("existing service missing %s", zhIndexPath)
		}
		patched, err := PatchServiceIndex(zhIndexBase, opt.Slug, zhTitle, zhOne, ZhCN)
		if err != nil {
			return nil, fmt.Errorf("patch zh index: %w", err)
		}
		if IsDestructiveUpdate(zhIndexBase, patched) {
			return nil, fmt.Errorf("refusing destructive zh index patch")
		}
		out = append(out, model.DocFileChange{Path: zhIndexPath, Action: "update", Content: patched})
	}

	return out, nil
}

// TitleFromFiles extracts `# title` from the practice markdown in files.
func TitleFromFiles(files []model.DocFileChange, docsRoot, service, slug string) string {
	want := filepath.ToSlash(filepath.Join(docsRoot, service, slug+".md"))
	for _, f := range files {
		if filepath.ToSlash(f.Path) != want {
			continue
		}
		if m := h1Re.FindStringSubmatch(f.Content); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return slug
}

// LoadBaselines returns zh-cn + en-us navigation files for prompt injection.
func LoadBaselines(cRepoRoot, _ignoredDocsRoot, service string) map[string]string {
	out := map[string]string{}
	for _, p := range []string{
		"docs/zh-cn/SUMMARY.md",
		"docs/en-us/SUMMARY.md",
		filepath.ToSlash(filepath.Join(zhDocsRoot, service, "index.md")),
		filepath.ToSlash(filepath.Join(enDocsRoot, service, "index.md")),
		filepath.ToSlash(filepath.Join(zhDocsRoot, "README.md")),
		filepath.ToSlash(filepath.Join(enDocsRoot, "README.md")),
	} {
		if s, ok := readFile(cRepoRoot, p); ok {
			out[p] = s
		}
	}
	return out
}

func readFile(root, rel string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", false
	}
	return string(data), true
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}
