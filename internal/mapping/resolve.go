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
	if slugMap, ok := r.slugsByService[cService]; ok {
		if actual, ok := slugMap[NormalizeKey(cSlug)]; ok {
			cSlug = actual
		} else if actual, ok := slugMap[NormalizeKey(bSlug)]; ok {
			cSlug = actual
		}
	} else if cService == "" {
		if actual, ok := r.flatSlugs[NormalizeKey(cSlug)]; ok {
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
	if target.Service == "" {
		if _, ok := r.flatSlugs[NormalizeKey(target.Slug)]; ok {
			return true
		}
		if _, ok := r.flatSlugs[NormalizeKey(p.Slug())]; ok {
			return true
		}
	}
	if slugMap, ok := r.slugsByService[target.Service]; ok {
		if _, ok := slugMap[NormalizeKey(target.Slug)]; ok {
			return true
		}
		if _, ok := slugMap[NormalizeKey(p.Slug())]; ok {
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
				if _, ok := slugMap[NormalizeKey(target.Slug)]; ok {
					return true
				}
				if _, ok := slugMap[NormalizeKey(p.Slug())]; ok {
					return true
				}
			}
		}
	}
	if r.docsAbs != "" {
		candidates := []string{
			filepath.Join(r.docsAbs, target.Service, target.Slug+".md"),
			filepath.Join(r.docsAbs, target.Slug+".md"),
		}
		if p.Service() != "" && p.Service() != target.Service {
			candidates = append(candidates, filepath.Join(r.docsAbs, p.Service(), PreferUnderscore(p.Slug())+".md"))
			candidates = append(candidates, filepath.Join(r.docsAbs, p.Service(), p.Slug()+".md"))
		}
		for _, c := range candidates {
			if c == "" || strings.Contains(c, string(filepath.Separator)+string(filepath.Separator)) {
				continue
			}
			if _, err := os.Stat(c); err == nil {
				return true
			}
		}
	}
	return false
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
