package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/chnsz/gitbook-practice-synchronization/internal/ai"
	"github.com/chnsz/gitbook-practice-synchronization/internal/ai/provider"
	"github.com/chnsz/gitbook-practice-synchronization/internal/config"
	"github.com/chnsz/gitbook-practice-synchronization/internal/gitops"
	"github.com/chnsz/gitbook-practice-synchronization/internal/mapping"
	"github.com/chnsz/gitbook-practice-synchronization/internal/model"
	"github.com/chnsz/gitbook-practice-synchronization/internal/monitor"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "detect":
		os.Exit(runDetect(args))
	case "generate":
		os.Exit(runGenerate(args))
	case "run":
		os.Exit(runPipeline(args))
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `gitbook-practice-synchronization — B examples → C docs PR (DeepSeek)

Usage:
  gitbook-practice-synchronization detect [--out FILE] [--no-refresh]
  gitbook-practice-synchronization generate [--practice ID] [--practices-file FILE] [--dry-run]
  gitbook-practice-synchronization run [--practice ID] [--dry-run] [--no-refresh]

Env:
  B_REPO          required, source examples repo (owner/name)
  C_REPO          required, docs / PR target repo (owner/name)
  MAX_PRACTICES   max new practices per run after filters (0 = unlimited)

Notes:
  Open tool PRs on C (branch prefix gitbook-practice-synchronization/) block
  their entire docs({service}); at most one practice per service is processed
  per scan until that PR is merged.
`)
}

func loadSettings() *config.Settings {
	s, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	return s
}

func runDetect(args []string) int {
	fs := flag.NewFlagSet("detect", flag.ExitOnError)
	out := fs.String("out", "", "write DetectionResult JSON")
	noRefresh := fs.Bool("no-refresh", false, "skip fetch/pull")
	_ = fs.Parse(args)

	s := loadSettings()
	if err := s.RequireRepos(); err != nil {
		log.Println(err)
		return 1
	}
	ctx, err := (&monitor.RepoWatcher{Settings: s}).PrepareRepos(!*noRefresh)
	if err != nil {
		log.Println(err)
		return 1
	}
	result, err := (&monitor.ChangeDetector{Settings: s}).Detect(ctx)
	if err != nil {
		log.Println(err)
		return 1
	}
	if err := filterOpenPRPractices(s, result); err != nil {
		log.Println(err)
		return 1
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	if *out != "" {
		if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
			log.Println(err)
			return 1
		}
		fmt.Printf("Wrote %s (%d new)\n", *out, len(result.NewPractices))
		return 0
	}
	fmt.Println(string(data))
	return 0
}

func runGenerate(args []string) int {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	practice := fs.String("practice", "", "single practice_id")
	practicesFile := fs.String("practices-file", "", "JSON from detect")
	dryRun := fs.Bool("dry-run", true, "do not push/create PR")
	_ = fs.Parse(args)

	s := loadSettings()
	s.DryRun = *dryRun
	if err := s.RequireRepos(); err != nil {
		log.Println(err)
		return 1
	}
	if err := s.RequireAI(); err != nil {
		log.Println(err)
		return 1
	}

	repoCtx, err := (&monitor.RepoWatcher{Settings: s}).PrepareRepos(true)
	if err != nil {
		log.Println(err)
		return 1
	}
	detection, err := (&monitor.ChangeDetector{Settings: s}).Detect(repoCtx)
	if err != nil {
		log.Println(err)
		return 1
	}
	if err := filterOpenPRPractices(s, detection); err != nil {
		log.Println(err)
		return 1
	}

	selected, err := selectPractices(s, repoCtx, detection, *practice, *practicesFile)
	if err != nil {
		log.Println(err)
		return 1
	}
	if len(selected) == 0 {
		fmt.Println("No practices to generate.")
		return 0
	}
	selected = limitPractices(selected, s.MaxPractices)

	p := provider.NewDeepSeek(s.AIAPIKey, s.AIBaseURL, s.AIModel, s.AITimeoutSeconds, s.AIMaxRetries, s.AIMaxTokens)
	gen := ai.NewDocGenerator(s, p)
	op := &gitops.RepoOperator{Settings: s}
	for _, item := range selected {
		if err := op.ResetToBase(repoCtx.C.LocalPath, s.CDefaultBranch); err != nil {
			log.Printf("reset C before %s: %v", item.PracticeID, err)
			return 1
		}
		dir := filepath.Join(repoCtx.B.LocalPath, item.SourcePath)
		result, err := gen.Generate(context.Background(), item, dir, repoCtx.C.LocalPath)
		if err != nil {
			log.Printf("generate %s: %v", item.PracticeID, err)
			return 1
		}
		fmt.Printf("Generated %s: %d file(s)\n", item.PracticeID, len(result.Files))
		for _, f := range result.Files {
			fmt.Printf("  - %s %s\n", f.Action, f.Path)
		}
	}
	return 0
}

func runPipeline(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	practice := fs.String("practice", "", "only this practice_id")
	dryRunFlag := fs.String("dry-run", "", "true|false override")
	noRefresh := fs.Bool("no-refresh", false, "skip fetch/pull")
	_ = fs.Parse(args)

	s := loadSettings()
	if *dryRunFlag != "" {
		s.DryRun = strings.EqualFold(*dryRunFlag, "true") || *dryRunFlag == "1"
	}
	if err := s.RequireRepos(); err != nil {
		log.Println(err)
		return 1
	}

	repoCtx, err := (&monitor.RepoWatcher{Settings: s}).PrepareRepos(!*noRefresh)
	if err != nil {
		log.Println(err)
		return 1
	}
	detection, err := (&monitor.ChangeDetector{Settings: s}).Detect(repoCtx)
	if err != nil {
		log.Println(err)
		return 1
	}
	if err := filterOpenPRPractices(s, detection); err != nil {
		log.Println(err)
		return 1
	}

	selected := detection.NewPractices
	if *practice != "" {
		var filtered []model.Practice
		for _, p := range selected {
			if p.PracticeID == *practice {
				filtered = append(filtered, p)
			}
		}
		selected = filtered
		if len(selected) == 0 {
			// Explicit practice may have been filtered as open-PR / one-per-service skip
			for _, msg := range detection.SkippedOpenPR {
				if strings.HasPrefix(msg, *practice+":") {
					fmt.Printf("Practice %s skipped: %s\n", *practice, strings.TrimPrefix(msg, *practice+": "))
					printJSON(model.PipelineResult{Detected: *detection, Skipped: detection.SkippedOpenPR, DryRun: s.DryRun})
					return 0
				}
			}
			fmt.Printf("No new practice matching %s\n", *practice)
			return 0
		}
	}

	pipeline := model.PipelineResult{
		Detected: *detection,
		Skipped:  append([]string{}, detection.SkippedOpenPR...),
		DryRun:   s.DryRun,
	}
	if len(selected) == 0 {
		fmt.Println("No new practices; exiting.")
		printJSON(pipeline)
		return 0
	}
	selected = limitPractices(selected, s.MaxPractices)
	if err := s.RequireAI(); err != nil {
		log.Println(err)
		return 1
	}

	p := provider.NewDeepSeek(s.AIAPIKey, s.AIBaseURL, s.AIModel, s.AITimeoutSeconds, s.AIMaxRetries, s.AIMaxTokens)
	gen := ai.NewDocGenerator(s, p)
	op := &gitops.RepoOperator{Settings: s}
	prm, err := gitops.NewPRManager(s)
	if err != nil {
		log.Println(err)
		return 1
	}
	stateMgr := &monitor.StateManager{Path: s.AbsoluteStatePath()}
	state, _ := stateMgr.Load()

	resolver := mapping.NewResolver(s.Mapping, s.CDocsRoot)
	openedServices := map[string]string{} // service → practice_id opened/kept this run

	for _, item := range selected {
		doc := resolver.Resolve(item)
		svc := strings.ToLower(strings.TrimSpace(doc.Service))
		if svc == "" {
			svc = strings.ToLower(strings.TrimSpace(item.Service()))
		}
		if svc == "" {
			svc = "unknown"
		}
		if prev, ok := openedServices[svc]; ok {
			msg := fmt.Sprintf("%s: skip, service %q already handled in this run (%s)", item.PracticeID, svc, prev)
			pipeline.Skipped = append(pipeline.Skipped, msg)
			continue
		}

		// Re-check immediately before generate (race with concurrent runs / prior opens)
		branch := gitops.PracticeBranch(item.PracticeID)
		if existing, err := prm.FindOpenPR(branch); err != nil {
			pipeline.Errors = append(pipeline.Errors, fmt.Sprintf("%s: %v", item.PracticeID, err))
			continue
		} else if existing != nil {
			msg := fmt.Sprintf("%s: skip, open PR #%d %s (head %s)", item.PracticeID, existing.Number, existing.URL, branch)
			pipeline.Skipped = append(pipeline.Skipped, msg)
			state.OpenPRs[item.PracticeID] = existing.URL
			openedServices[svc] = item.PracticeID
			continue
		}

		// Always start Generate from a clean C base. After a prior ApplyAndPush in
		// this run the worktree sits on that practice branch; reading SUMMARY/index
		// from there would bake sibling-service nav into this PR (see hcbp-demo#18).
		if err := op.ResetToBase(repoCtx.C.LocalPath, s.CDefaultBranch); err != nil {
			pipeline.Errors = append(pipeline.Errors, fmt.Sprintf("%s reset C: %v", item.PracticeID, err))
			continue
		}

		dir := filepath.Join(repoCtx.B.LocalPath, item.SourcePath)
		result, err := gen.Generate(context.Background(), item, dir, repoCtx.C.LocalPath)
		if err != nil {
			pipeline.Errors = append(pipeline.Errors, fmt.Sprintf("%s: %v", item.PracticeID, err))
			continue
		}
		pipeline.Generated = append(pipeline.Generated, *result)

		title := commitTitle(s, item)
		tpl, _ := gitops.LoadPRBodyTemplate(s.RepoRoot)
		body := gitops.BuildPRBody(gitops.PRBodyInput{
			Practice:   item,
			Result:     result,
			BRepo:      s.BRepo,
			CRepo:      s.CRepo,
			BSHA:       detection.BCommit,
			SkillID:    s.SkillID,
			AIProvider: aiProviderLabel(s.AIModel),
			Template:   tpl,
		})

		if _, err := op.ApplyAndPush(repoCtx.C.LocalPath, branch, s.CDefaultBranch, result, title, s.DryRun); err != nil {
			pipeline.Errors = append(pipeline.Errors, fmt.Sprintf("%s push: %v", item.PracticeID, err))
			continue
		}
		pr, err := prm.CreatePR(title, body, branch, s.CDefaultBranch, s.DryRun)
		if err != nil {
			pipeline.Errors = append(pipeline.Errors, fmt.Sprintf("%s pr: %v", item.PracticeID, err))
			continue
		}
		if pr != nil && pr.URL != "" {
			pipeline.PRURLs = append(pipeline.PRURLs, pr.URL)
			state.OpenPRs[item.PracticeID] = pr.URL
		}
		openedServices[svc] = item.PracticeID
		if !contains(state.ProcessedPractices, item.PracticeID) {
			state.ProcessedPractices = append(state.ProcessedPractices, item.PracticeID)
		}
	}

	state.BCommit = detection.BCommit
	state.CCommit = detection.CCommit
	if !s.DryRun {
		_ = stateMgr.Save(state)
	}
	printJSON(pipeline)
	if len(pipeline.Errors) > 0 {
		return 1
	}
	return 0
}

func limitPractices(practices []model.Practice, max int) []model.Practice {
	if max <= 0 || len(practices) <= max {
		return practices
	}
	fmt.Printf("Limiting practices: %d → %d (MAX_PRACTICES)\n", len(practices), max)
	return practices[:max]
}

func selectPractices(s *config.Settings, ctx *monitor.RepoContext, detection *model.DetectionResult, practice, practicesFile string) ([]model.Practice, error) {
	if practice != "" {
		for _, p := range detection.NewPractices {
			if p.PracticeID == practice {
				return []model.Practice{p}, nil
			}
		}
		all, err := monitor.EnumeratePractices(
			filepath.Join(ctx.B.LocalPath, s.BExamplesPath),
			s.BExamplesPath,
			s.IgnoreNames,
			s.Granularity,
		)
		if err != nil {
			return nil, err
		}
		for _, p := range all {
			if p.PracticeID == practice {
				return []model.Practice{p}, nil
			}
		}
		return nil, fmt.Errorf("practice not found: %s", practice)
	}
	if practicesFile != "" {
		data, err := os.ReadFile(practicesFile)
		if err != nil {
			return nil, err
		}
		var parsed model.DetectionResult
		if err := json.Unmarshal(data, &parsed); err != nil {
			return nil, err
		}
		want := map[string]struct{}{}
		for _, p := range parsed.NewPractices {
			want[p.PracticeID] = struct{}{}
		}
		var selected []model.Practice
		for _, p := range detection.NewPractices {
			if _, ok := want[p.PracticeID]; ok {
				selected = append(selected, p)
			}
		}
		return selected, nil
	}
	return detection.NewPractices, nil
}

func filterOpenPRPractices(s *config.Settings, detection *model.DetectionResult) error {
	if detection == nil || len(detection.NewPractices) == 0 {
		return nil
	}
	if strings.TrimSpace(s.CRepoToken) == "" {
		fmt.Println("warn: C_REPO_TOKEN unset; skip open-PR filter")
		return nil
	}
	prm, err := gitops.NewPRManager(s)
	if err != nil {
		return err
	}
	resolver := mapping.NewResolver(s.Mapping, s.CDocsRoot)
	serviceOf := func(p model.Practice) string {
		doc := resolver.Resolve(p)
		svc := strings.ToLower(strings.TrimSpace(doc.Service))
		if svc == "" {
			svc = strings.ToLower(strings.TrimSpace(p.Service()))
		}
		if svc == "" {
			return "unknown"
		}
		return svc
	}
	keep, skipped, err := prm.FilterOpenPRs(detection.NewPractices, serviceOf)
	if err != nil {
		return err
	}
	detection.NewPractices = keep
	if len(skipped) > 0 {
		detection.SkippedOpenPR = append(detection.SkippedOpenPR, skipped...)
		fmt.Printf("Skipped %d practice(s) due to open PR / one-per-service on %s\n", len(skipped), s.CRepo)
	}
	return nil
}

func commitTitle(s *config.Settings, p model.Practice) string {
	doc := mapping.NewResolver(s.Mapping, s.CDocsRoot).Resolve(p)
	service := doc.Service
	if service == "" {
		service = p.Service()
	}
	if service == "" {
		service = "unknown"
	}
	slug := doc.Slug
	if slug == "" {
		slug = p.Slug()
	}
	return fmt.Sprintf("docs(%s): support new best practice for %s", service, model.SimplePracticeTitle(service, slug))
}

func aiProviderLabel(modelName string) string {
	m := strings.ToLower(strings.TrimSpace(modelName))
	switch {
	case m == "":
		return "AI"
	case strings.Contains(m, "deepseek"):
		return "DeepSeek"
	default:
		return modelName
	}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func printJSON(v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}
