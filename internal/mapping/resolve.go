package mapping

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chnsz/gitbook-practice-synchronization/internal/config"
	"github.com/chnsz/gitbook-practice-synchronization/internal/model"
)

// DocTarget is the canonical location under C repo docs.
type DocTarget struct {
	Service string // C service dir, e.g. anti-ddos
	Slug    string // C practice file stem (may contain / for nested docs), e.g. kafka/instance_configuration
	RelPath string // docs/zh-cn/best-practices/{service}/{slug}.md
}

// sourceDoc locates a C practice file discovered via B examples/ source link.
type sourceDoc struct {
	Service string
	Slug    string
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
	// sourceByKey maps B practice paths found in C doc Reference links → existing C slug.
	// Keys are lowercase practice ids such as examples/ecs/attached-volume.
	sourceByKey map[string]sourceDoc
}

var (
	// Match GitHub tree/blob links that point at B-repo examples/... paths.
	examplesTreeLinkRe = regexp.MustCompile(`(?i)(?:github\.com/[^/]+/[^/]+)/(?:tree|blob)/[^/\s)"']+/(examples/[a-z0-9_./-]+)`)
)

// NewResolver builds a resolver from mapping config.
func NewResolver(cfg config.MappingConfig, docsRoot string) *Resolver {
	r := &Resolver{
		DocsRoot:        docsRoot,
		ServiceAliases:  map[string]string{},
		PracticeAliases: map[string]string{},
		serviceByNorm:   map[string]string{},
		slugsByService:  map[string]map[string]string{},
		flatSlugs:       map[string]string{},
		sourceByKey:     map[string]sourceDoc{},
	}
	for k, v := range cfg.ServiceAliases {
		r.ServiceAliases[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	for k, v := range cfg.PracticeAliases {
		r.PracticeAliases[normalizeAliasKey(k)] = strings.TrimSpace(v)
	}
	return r
}

// IndexDocsRoot scans C docs for existence checks (service dirs + nested .md + flat .md).
func (r *Resolver) IndexDocsRoot(cRepoRoot string) error {
	root := filepath.Join(cRepoRoot, r.DocsRoot)
	r.docsAbs = root
	r.sourceByKey = map[string]sourceDoc{}
	r.serviceByNorm = map[string]string{}
	r.slugsByService = map[string]map[string]string{}
	r.flatSlugs = map[string]string{}
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
					r.indexSourceLinks("", base, filepath.Join(root, name))
				}
			}
			continue
		}
		svc := name
		r.serviceByNorm[NormalizeKey(svc)] = svc
		slugMap := map[string]string{}
		svcDir := filepath.Join(root, svc)
		_ = filepath.WalkDir(svcDir, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return walkErr
			}
			if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
				return nil
			}
			rel, err := filepath.Rel(svcDir, path)
			if err != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			base := strings.TrimSuffix(rel, filepath.Ext(rel))
			leaf := filepath.Base(base)
			if strings.EqualFold(leaf, "index") || strings.EqualFold(leaf, "readme") {
				return nil
			}
			slugMap[NormalizeKey(base)] = base
			r.indexSourceLinks(svc, base, path)
			return nil
		})
		r.slugsByService[svc] = slugMap
	}
	return nil
}

func (r *Resolver) indexSourceLinks(service, slug, absPath string) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return
	}
	for _, id := range extractExamplePracticeIDs(string(data)) {
		for _, key := range sourceLookupKeys(id) {
			if key == "" {
				continue
			}
			if existing, exists := r.sourceByKey[key]; exists {
				// Prefer the longer/more specific KEEP slug when duplicates both link the same B path.
				if len(NormalizeKey(slug)) <= len(NormalizeKey(existing.Slug)) {
					continue
				}
			}
			r.sourceByKey[key] = sourceDoc{Service: service, Slug: slug}
		}
	}
}

func extractExamplePracticeIDs(content string) []string {
	var out []string
	seen := map[string]struct{}{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		id = strings.TrimSuffix(id, "/")
		if id == "" || !strings.HasPrefix(strings.ToLower(id), "examples/") {
			return
		}
		key := strings.ToLower(id)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, id)
	}
	for _, m := range examplesTreeLinkRe.FindAllStringSubmatch(content, -1) {
		add(m[1])
	}
	return out
}

func sourceLookupKeys(practiceID string) []string {
	practiceID = strings.TrimSpace(practiceID)
	if practiceID == "" {
		return nil
	}
	lower := strings.ToLower(strings.TrimSuffix(practiceID, "/"))
	underscore := preferUnderscorePath(lower)
	keys := []string{lower, underscore, normalizeAliasKey(lower), normalizeAliasKey(underscore)}
	seen := map[string]struct{}{}
	var out []string
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func preferUnderscorePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = PreferUnderscore(part)
	}
	return strings.Join(parts, "/")
}

func (r *Resolver) lookupBySource(p model.Practice) (sourceDoc, bool) {
	for _, raw := range []string{p.PracticeID, p.SourcePath} {
		for _, key := range sourceLookupKeys(raw) {
			if doc, ok := r.sourceByKey[key]; ok {
				return doc, true
			}
		}
	}
	return sourceDoc{}, false
}

// Resolve returns canonical C doc target for a B practice.
func (r *Resolver) Resolve(p model.Practice) DocTarget {
	bService := p.Service()
	bRel := p.RelUnderService()
	bSlug := p.Slug()
	cService := r.CanonicalService(bService)
	cSlug := r.CanonicalSlug(p.PracticeID, bService, bRel, bSlug)

	// Prefer an already-published C doc that links back to this B examples path
	// (covers renames like attached-volume → instance_with_volume).
	if doc, ok := r.lookupBySource(p); ok {
		if doc.Service != "" {
			cService = doc.Service
		}
		if doc.Slug != "" {
			cSlug = doc.Slug
		}
	}

	if actualSvc, ok := r.serviceByNorm[NormalizeKey(cService)]; ok {
		cService = actualSvc
	}
	cands := SlugCandidates(bService, bRel)
	if slugMap, ok := r.slugsByService[cService]; ok {
		cSlug = pickIndexedSlug(slugMap, cSlug, bService, cands...)
	} else if cService == "" {
		cSlug = pickIndexedSlug(r.flatSlugs, cSlug, bService, cands...)
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

// CanonicalSlug maps B practice path under service to C markdown stem (may contain /).
func (r *Resolver) CanonicalSlug(practiceID, bService, bRel, bSlug string) string {
	bRel = strings.TrimSpace(bRel)
	if bRel == "" {
		bRel = bSlug
	}
	for _, key := range []string{
		normalizeAliasKey(practiceID),
		normalizeAliasKey(bService + "/" + bRel),
		normalizeAliasKey(bService + "/" + bSlug),
		normalizeAliasKey(bRel),
		normalizeAliasKey(bSlug),
	} {
		if alias, ok := r.PracticeAliases[key]; ok && alias != "" {
			return preferUnderscorePath(alias)
		}
	}
	return preferUnderscorePath(bRel)
}

// IsSynced reports whether a matching doc already exists under C.
func (r *Resolver) IsSynced(p model.Practice) bool {
	if _, ok := r.lookupBySource(p); ok {
		return true
	}
	target := r.Resolve(p)
	cands := []string{target.Slug}
	for _, c := range SlugCandidates(p.Service(), p.RelUnderService()) {
		if isOversimplifiedCandidate(c, target.Slug, p.Service()) {
			continue
		}
		cands = append(cands, c)
	}
	for _, c := range slugOrthography(target.Slug) {
		cands = append(cands, c)
	}

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
			filepath.Join(r.docsAbs, target.Service, filepath.FromSlash(target.Slug)+".md"),
			filepath.Join(r.docsAbs, filepath.FromSlash(target.Slug)+".md"),
		}
		for _, cand := range cands {
			stem := preferUnderscorePath(cand)
			if target.Service != "" {
				paths = append(paths, filepath.Join(r.docsAbs, target.Service, filepath.FromSlash(stem)+".md"))
			}
			paths = append(paths, filepath.Join(r.docsAbs, filepath.FromSlash(stem)+".md"))
			if p.Service() != "" && p.Service() != target.Service {
				paths = append(paths, filepath.Join(r.docsAbs, p.Service(), filepath.FromSlash(stem)+".md"))
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

// isOversimplifiedCandidate reports Path A-style short names that must not count
// as synced when the canonical KEEP slug is more specific.
func isOversimplifiedCandidate(cand, canonical, service string) bool {
	a, b := NormalizeKey(cand), NormalizeKey(canonical)
	if a == "" || b == "" || a == b {
		return false
	}
	canPath := preferUnderscorePath(canonical)
	if strings.Contains(canPath, "/") && a == NormalizeKey(filepath.Base(canPath)) {
		return true
	}
	// Much shorter than canonical (zone vs public_zone, custom vs event_subscription_*).
	if len(a)*2 <= len(b) {
		return true
	}
	if strings.HasSuffix(b, a) && len(b) > len(a) {
		leaf := filepath.Base(canPath)
		parts := splitSlugParts(leaf)
		if len(parts) >= 2 && isShortProductToken(parts[0]) && NormalizeKey(strings.Join(parts[1:], "_")) == a {
			// kps_keypair → keypair is a valid C rename (product code ≠ service).
			// ddm_account → account strips the service name itself → Path A.
			if service != "" && NormalizeKey(parts[0]) == NormalizeKey(service) {
				return true
			}
			return false
		}
		return true
	}
	return false
}

func slugOrthography(slug string) []string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil
	}
	u := preferUnderscorePath(slug)
	h := strings.ReplaceAll(u, "_", "-")
	return []string{u, h, PreferUnderscore(slug)}
}

// SlugCandidates returns normalized lookup forms for a B practice slug.
// Includes PreferUnderscore(slug) and, when the first segment looks like a short
// product code (e.g. kps-keypair → keypair), the slug with that prefix dropped.
// This mirrors C-repo naming where Deploy Keypair lives at keypair.md, not kps_keypair.md.
func SlugCandidates(service, slug string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		s = preferUnderscorePath(strings.TrimSpace(s))
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
	if base := filepath.Base(strings.ReplaceAll(slug, "\\", "/")); base != "" && base != slug {
		add(base)
	}
	if service != "" {
		add(stripLeadingServiceToken(slug, service))
		if base := filepath.Base(preferUnderscorePath(slug)); base != slug {
			add(stripLeadingServiceToken(base, service))
		}
	}

	u := preferUnderscorePath(slug)
	leaf := filepath.Base(u)
	parts := splitSlugParts(leaf)
	if len(parts) >= 2 && isShortProductToken(parts[0]) {
		rest := strings.Join(parts[1:], "_")
		// Avoid over-aggressive matches like kms_key → key (too short).
		if len(NormalizeKey(rest)) >= 4 {
			add(rest)
		}
	}

	// Common C-repo rename: B attached-volume ↔ C instance_with_volume
	if rest, ok := strings.CutPrefix(leaf, "attached_"); ok && rest != "" {
		add("instance_with_" + rest)
	}
	if rest, ok := strings.CutPrefix(leaf, "instance_with_"); ok && rest != "" {
		add("attached_" + rest)
	}

	// redis_ha_* ↔ redis_high_availability_*
	for _, form := range expandAvailabilityAbbrev(leaf) {
		add(form)
	}
	return out
}

func expandAvailabilityAbbrev(u string) []string {
	u = PreferUnderscore(u)
	var out []string
	if strings.Contains(u, "high_availability") {
		out = append(out, strings.ReplaceAll(u, "high_availability", "ha"))
	}
	parts := strings.Split(u, "_")
	changed := false
	var rebuilt []string
	for _, p := range parts {
		if p == "ha" {
			rebuilt = append(rebuilt, "high", "availability")
			changed = true
			continue
		}
		rebuilt = append(rebuilt, p)
	}
	if changed {
		out = append(out, strings.Join(rebuilt, "_"))
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

// pickIndexedSlug keeps the canonical stem when present; otherwise adopts a
// rename/product-prefix match. Never demotes to oversimplified Path A names.
func pickIndexedSlug(slugMap map[string]string, canonical, service string, candidates ...string) string {
	if slugMap == nil {
		return canonical
	}
	if actual, ok := slugMap[NormalizeKey(canonical)]; ok {
		return actual
	}
	var best string
	bestLen := -1
	seen := map[string]struct{}{}
	try := func(c string) {
		c = strings.TrimSpace(c)
		if c == "" {
			return
		}
		if isOversimplifiedCandidate(c, canonical, service) {
			return
		}
		actual, ok := slugMap[NormalizeKey(c)]
		if !ok {
			return
		}
		if isOversimplifiedCandidate(actual, canonical, service) {
			return
		}
		k := NormalizeKey(actual)
		if _, dup := seen[k]; dup {
			return
		}
		seen[k] = struct{}{}
		if n := len(k); n > bestLen {
			bestLen = n
			best = actual
		}
	}
	try(canonical)
	for _, c := range candidates {
		try(c)
	}
	if best != "" {
		return best
	}
	return canonical
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
	repl := strings.NewReplacer("-", "", "_", "", " ", "", ".", "", "/", "")
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
