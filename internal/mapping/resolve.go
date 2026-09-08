package mapping

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/chnsz/gitbook-practice-synchronization/internal/config"
	"github.com/chnsz/gitbook-practice-synchronization/internal/model"
)

// DocTarget is the canonical location under C repo docs.
type DocTarget struct {
	Service string // C service dir, e.g. anti-ddos
	Slug    string // C practice file stem, e.g. default_protection_policy
	RelPath string // docs/zh-cn/best-practices/{service}/{slug}.md
}

// Resolver maps B examples paths onto C hcbp-demo doc paths.
type Resolver struct {
	DocsRoot        string
	ServiceAliases  map[string]string // B service → C service
	PracticeAliases map[string]string // B practice_id or service/slug → C slug
	docsAbs         string
	serviceByNorm   map[string]string            // normalized → actual C service dir
	slugsByService  map[string]map[string]string // C service → (normalized slug → actual slug)
	flatSlugs       map[string]string            // normalized → stem for .md directly under DocsRoot
}

// NewResolver builds a resolver from mapping config.
func NewResolver(cfg config.MappingConfig, docsRoot string) *Resolver {
	r := &Resolver{
		DocsRoot:        docsRoot,
		ServiceAliases:  map[string]string{},
		PracticeAliases: map[string]string{},
		serviceByNorm:   map[string]string{},
		slugsByService:  map[string]map[string]string{},
		flatSlugs:       map[string]string{},
	}
	for k, v := range cfg.ServiceAliases {
		r.ServiceAliases[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	for k, v := range cfg.PracticeAliases {
		r.PracticeAliases[normalizeAliasKey(k)] = strings.TrimSpace(v)
	}
	return r
}

// IndexDocsRoot scans C docs for existence checks (service dirs + flat .md).
func (r *Resolver) IndexDocsRoot(cRepoRoot string) error {
	root := filepath.Join(cRepoRoot, r.DocsRoot)
	r.docsAbs = root
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if !e.IsDir() {
			if strings.HasSuffix(strings.ToLower(name), ".md") {
				base := strings.TrimSuffix(name, filepath.Ext(name))
				if !strings.EqualFold(base, "index") && !strings.EqualFold(base, "readme") {
					r.flatSlugs[NormalizeKey(base)] = base
				}
			}
			continue
		}
		svc := name
		r.serviceByNorm[NormalizeKey(svc)] = svc
		slugMap := map[string]string{}
		files, err := os.ReadDir(filepath.Join(root, svc))
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(strings.ToLower(f.Name()), ".md") {
				continue
			}
			base := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
			if strings.EqualFold(base, "index") || strings.EqualFold(base, "readme") {
				continue
			}
			slugMap[NormalizeKey(base)] = base
		}
		r.slugsByService[svc] = slugMap
	}
	return nil
}

// Resolve returns canonical C doc target for a B practice.
func (r *Resolver) Resolve(p model.Practice) DocTarget {
	bService := p.Service()
	bSlug := p.Slug()
	cService := r.CanonicalService(bService)
	cSlug := r.CanonicalSlug(p.PracticeID, bService, bSlug)

	if actualSvc, ok := r.serviceByNorm[NormalizeKey(cService)]; ok {
		cService = actualSvc
	}
	cands := SlugCandidates(bService, bSlug)
	cands = append(cands, cSlug)
	if slugMap, ok := r.slugsByService[cService]; ok {
		if actual := lookupSlug(slugMap, cands...); actual != "" {
			cSlug = actual
		}
	} else if cService == "" {
		if actual := lookupSlug(r.flatSlugs, cands...); actual != "" {
			cSlug = actual
		}
	}

	var rel string
	if cService == "" {
		rel = filepath.ToSlash(filepath.Join(r.DocsRoot, cSlug+".md"))
	} else {
		rel = filepath.ToSlash(filepath.Join(r.DocsRoot, cService, cSlug+".md"))
	}
	return DocTarget{Service: cService, Slug: cSlug, RelPath: rel}
}

// CanonicalService maps B examples service dir to C docs service dir.
func (r *Resolver) CanonicalService(bService string) string {
	if bService == "" {
		return bService
	}
	key := strings.ToLower(bService)
	if alias, ok := r.ServiceAliases[key]; ok && alias != "" {
		return alias
	}
	if actual, ok := r.serviceByNorm[NormalizeKey(bService)]; ok {
		return actual
	}
	return bService
}

// CanonicalSlug maps B practice directory name to C markdown stem.
func (r *Resolver) CanonicalSlug(practiceID, bService, bSlug string) string {
	for _, key := range []string{
		normalizeAliasKey(practiceID),
		normalizeAliasKey(bService + "/" + bSlug),
		normalizeAliasKey(bSlug),
	} {
		if alias, ok := r.PracticeAliases[key]; ok && alias != "" {
			return PreferUnderscore(alias)
		}
	}
	return PreferUnderscore(bSlug)
}

// IsSynced reports whether a matching doc already exists under C.
func (r *Resolver) IsSynced(p model.Practice) bool {
	target := r.Resolve(p)
	cands := SlugCandidates(p.Service(), p.Slug())
	cands = append(cands, target.Slug)

	if target.Service == "" {
		if lookupSlug(r.flatSlugs, cands...) != "" {
			return true
		}
	}
	if slugMap, ok := r.slugsByService[target.Service]; ok {
		if lookupSlug(slugMap, cands...) != "" {
			return true
		}
	}
	// Fuzzy: any indexed service matching normalized B/C service name
	for _, key := range []string{NormalizeKey(target.Service), NormalizeKey(p.Service())} {
		if key == "" {
			continue
		}
		if svc, ok := r.serviceByNorm[key]; ok {
			if slugMap := r.slugsByService[svc]; slugMap != nil {
				if lookupSlug(slugMap, cands...) != "" {
					return true
				}
			}
		}
	}
	if r.docsAbs != "" {
		paths := []string{
			filepath.Join(r.docsAbs, target.Service, target.Slug+".md"),
			filepath.Join(r.docsAbs, target.Slug+".md"),
		}
		for _, cand := range cands {
			stem := PreferUnderscore(cand)
			if target.Service != "" {
				paths = append(paths, filepath.Join(r.docsAbs, target.Service, stem+".md"))
			}
			paths = append(paths, filepath.Join(r.docsAbs, stem+".md"))
			if p.Service() != "" && p.Service() != target.Service {
				paths = append(paths, filepath.Join(r.docsAbs, p.Service(), stem+".md"))
			}
		}
		seen := map[string]struct{}{}
		for _, c := range paths {
			if c == "" || strings.Contains(c, string(filepath.Separator)+string(filepath.Separator)) {
				continue
			}
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			if _, err := os.Stat(c); err == nil {
				return true
			}
		}
	}
	return false
}

// SlugCandidates returns normalized lookup forms for a B practice slug.
// Includes PreferUnderscore(slug) and, when the first segment looks like a short
// product code (e.g. kps-keypair → keypair), the slug with that prefix dropped.
// This mirrors C-repo naming where Deploy Keypair lives at keypair.md, not kps_keypair.md.
func SlugCandidates(service, slug string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		s = PreferUnderscore(strings.TrimSpace(s))
		if s == "" {
			return
		}
		k := NormalizeKey(s)
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		out = append(out, s)
	}

	add(slug)
	if service != "" {
		add(stripLeadingServiceToken(slug, service))
	}

	parts := splitSlugParts(PreferUnderscore(slug))
	if len(parts) >= 2 && isShortProductToken(parts[0]) {
		rest := strings.Join(parts[1:], "_")
		// Avoid over-aggressive matches like kms_key → key (too short).
		if len(NormalizeKey(rest)) >= 4 {
			add(rest)
		}
	}
	return out
}

func lookupSlug(slugMap map[string]string, candidates ...string) string {
	for _, c := range candidates {
		if c == "" || slugMap == nil {
			continue
		}
		if actual, ok := slugMap[NormalizeKey(c)]; ok {
			return actual
		}
	}
	return ""
}

func splitSlugParts(slug string) []string {
	slug = PreferUnderscore(slug)
	if slug == "" {
		return nil
	}
	return strings.Split(slug, "_")
}

func isShortProductToken(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if n := len(s); n < 2 || n > 5 {
		return false
	}
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// stripLeadingServiceToken drops a leading service name from slug when present
// (e.g. ecs-simple-instance under ecs → simple-instance).
func stripLeadingServiceToken(slug, service string) string {
	slug = strings.TrimSpace(slug)
	service = strings.TrimSpace(service)
	if slug == "" || service == "" {
		return slug
	}
	variants := []string{
		service,
		strings.ReplaceAll(service, "-", "_"),
		strings.ReplaceAll(service, "_", "-"),
		NormalizeKey(service),
	}
	lower := strings.ToLower(slug)
	for _, v := range variants {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		for _, sep := range []string{"_", "-"} {
			prefix := v + sep
			if strings.HasPrefix(lower, prefix) && len(slug) > len(prefix) {
				return slug[len(prefix):]
			}
		}
	}
	return slug
}

// NormalizeKey removes separators for fuzzy comparison.
func NormalizeKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	repl := strings.NewReplacer("-", "", "_", "", " ", "", ".", "")
	return repl.Replace(s)
}

// PreferUnderscore converts hyphenated names to hcbp-demo style underscores.
func PreferUnderscore(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}

func normalizeAliasKey(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "./")
	s = strings.ToLower(s)
	return s
}
