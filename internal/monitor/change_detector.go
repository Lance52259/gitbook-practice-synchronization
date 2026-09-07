package monitor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chnsz/gitbook-practice-synchronization/internal/config"
	"github.com/chnsz/gitbook-practice-synchronization/internal/mapping"
	"github.com/chnsz/gitbook-practice-synchronization/internal/model"
)

// EnumeratePractices lists practices under examples/.
// granularity:
//   - "directory": first-level dirs are practices
//   - "nested_directory" (default for hcbp): examples/{service}/{practice}/
//     If a practice dir only contains nested leaf dirs with .tf, those leaves are enumerated
//     (e.g. examples/aom/alarm-rule/distribute-alarm).
func EnumeratePractices(examplesRoot, examplesRel string, ignoreNames []string, granularity string) ([]model.Practice, error) {
	ignore := map[string]struct{}{}
	for _, n := range ignoreNames {
		ignore[n] = struct{}{}
	}

	entries, err := os.ReadDir(examplesRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var practices []model.Practice
	for _, e := range entries {
		name := e.Name()
		if _, ok := ignore[name]; ok || strings.HasPrefix(name, ".") {
			continue
		}
		if !e.IsDir() {
			continue
		}
		serviceDir := filepath.Join(examplesRoot, name)
		if granularity == "nested_directory" {
			subs, err := os.ReadDir(serviceDir)
			if err != nil {
				continue
			}
			hasNested := false
			for _, sub := range subs {
				subName := sub.Name()
				if _, ok := ignore[subName]; ok || strings.HasPrefix(subName, ".") || !sub.IsDir() {
					continue
				}
				hasNested = true
				practiceDir := filepath.Join(serviceDir, subName)
				baseRel := strings.TrimRight(examplesRel, "/") + "/" + name + "/" + subName
				practices = append(practices, expandPracticeOrLeaves(practiceDir, baseRel, subName, ignore)...)
			}
			// flat tf under service dir (no nested practice folders) → treat service as one practice
			if !hasNested {
				rel := strings.TrimRight(examplesRel, "/") + "/" + name
				practices = append(practices, buildPractice(serviceDir, rel, name))
			}
			continue
		}

		rel := strings.TrimRight(examplesRel, "/") + "/" + name
		practices = append(practices, buildPractice(serviceDir, rel, name))
	}
	sort.Slice(practices, func(i, j int) bool {
		return practices[i].PracticeID < practices[j].PracticeID
	})
	return practices, nil
}

// expandPracticeOrLeaves emits nested leaf practices when the directory is a container
// (child dirs with Terraform, little/no .tf at this level).
func expandPracticeOrLeaves(dir, rel, name string, ignore map[string]struct{}) []model.Practice {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []model.Practice{buildPractice(dir, rel, name)}
	}
	var leafDirs []os.DirEntry
	for _, e := range entries {
		n := e.Name()
		if _, ok := ignore[n]; ok || strings.HasPrefix(n, ".") || !e.IsDir() {
			continue
		}
		child := filepath.Join(dir, n)
		if hasTerraformFiles(child) {
			leafDirs = append(leafDirs, e)
		}
	}
	if len(leafDirs) > 0 && !hasTerraformFiles(dir) {
		out := make([]model.Practice, 0, len(leafDirs))
		for _, e := range leafDirs {
			childRel := rel + "/" + e.Name()
			out = append(out, buildPractice(filepath.Join(dir, e.Name()), childRel, e.Name()))
		}
		return out
	}
	return []model.Practice{buildPractice(dir, rel, name)}
}

func hasTerraformFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := strings.ToLower(e.Name())
		if strings.HasSuffix(n, ".tf") || strings.HasSuffix(n, ".tf.json") {
			return true
		}
	}
	return false
}

func buildPractice(dir, rel, name string) model.Practice {
	files, _ := listFilesRel(dir)
	title := strings.ReplaceAll(strings.ReplaceAll(name, "-", " "), "_", " ")
	readme := filepath.Join(dir, "README.md")
	if data, err := os.ReadFile(readme); err == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "#") {
			t := strings.TrimSpace(strings.TrimLeft(lines[0], "#"))
			if t != "" {
				title = t
			}
		}
	}
	return model.Practice{
		PracticeID: rel,
		SourcePath: rel,
		Title:      title,
		Files:      files,
	}
}

func listFilesRel(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files, err
}

func loadManifestIDs(cRoot, manifestRel string) (map[string]struct{}, error) {
	empty := map[string]struct{}{}
	if strings.TrimSpace(manifestRel) == "" {
		return empty, nil
	}
	path := filepath.Join(cRoot, manifestRel)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return empty, nil
		}
		return nil, err
	}
	if info.IsDir() {
		return empty, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	ids := map[string]struct{}{}
	var asMap map[string]any
	if err := json.Unmarshal(data, &asMap); err == nil {
		for _, key := range []string{"practices", "synced", "ids"} {
			if raw, ok := asMap[key]; ok {
				addManifestItems(ids, raw)
			}
		}
		return ids, nil
	}
	var asList []any
	if err := json.Unmarshal(data, &asList); err == nil {
		addManifestItems(ids, asList)
		return ids, nil
	}
	return ids, nil
}

func addManifestItems(ids map[string]struct{}, raw any) {
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			switch t := item.(type) {
			case string:
				ids[t] = struct{}{}
			case map[string]any:
				if id, ok := t["id"].(string); ok {
					ids[id] = struct{}{}
				}
				if id, ok := t["practice_id"].(string); ok {
					ids[id] = struct{}{}
				}
			}
		}
	}
}

func inferSyncedByDocs(resolver *mapping.Resolver, practices []model.Practice) map[string]struct{} {
	synced := map[string]struct{}{}
	for _, p := range practices {
		if resolver.IsSynced(p) {
			synced[p.PracticeID] = struct{}{}
		}
	}
	return synced
}

// ChangeDetector computes new practices relative to C synced set.
type ChangeDetector struct {
	Settings *config.Settings
}

func (d *ChangeDetector) Detect(ctx *RepoContext) (*model.DetectionResult, error) {
	examplesRoot := filepath.Join(ctx.B.LocalPath, d.Settings.BExamplesPath)
	granularity := d.Settings.Granularity
	if granularity == "" {
		granularity = "nested_directory"
	}
	practices, err := EnumeratePractices(examplesRoot, d.Settings.BExamplesPath, d.Settings.IgnoreNames, granularity)
	if err != nil {
		return nil, err
	}

	manifestIDs, err := loadManifestIDs(ctx.C.LocalPath, d.Settings.CSyncedManifest)
	if err != nil {
		return nil, err
	}

	resolver := mapping.NewResolver(d.Settings.Mapping, d.Settings.CDocsRoot)
	if err := resolver.IndexDocsRoot(ctx.C.LocalPath); err != nil {
		return nil, err
	}
	pathIDs := inferSyncedByDocs(resolver, practices)

	synced := map[string]struct{}{}
	switch d.Settings.SyncedStrategy {
	case "manifest":
		synced = manifestIDs
	case "path_infer":
		synced = pathIDs
	default:
		for k := range manifestIDs {
			synced[k] = struct{}{}
		}
		for k := range pathIDs {
			synced[k] = struct{}{}
		}
	}

	var news []model.Practice
	for _, p := range practices {
		if _, ok := synced[p.PracticeID]; !ok {
			news = append(news, p)
		}
	}
	syncedList := make([]string, 0, len(synced))
	for id := range synced {
		syncedList = append(syncedList, id)
	}
	sort.Strings(syncedList)

	return &model.DetectionResult{
		NewPractices: news,
		SyncedIDs:    syncedList,
		BCommit:      ctx.B.CommitSHA,
		CCommit:      ctx.C.CommitSHA,
	}, nil
}
