package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Settings holds runtime configuration. Env/Secrets override YAML defaults.
type Settings struct {
	RepoRoot string

	BRepo          string
	BRepoToken     string
	BExamplesPath  string
	BDefaultBranch string
	IgnoreNames    []string

	CRepo           string
	CRepoToken      string
	CDocsRoot       string
	CDefaultBranch  string
	CSyncedManifest string
	PathAllowlist   []string

	AIBaseURL          string
	AIAPIKey           string
	AIModel            string
	AITimeoutSeconds   int
	AIMaxRetries       int
	AIMaxTokens        int
	MaxContextChars    int
	ResponseFormatJSON bool

	SkillID   string
	SkillRoot string

	DryRun       bool
	StatePath    string
	WorkDir      string
	PRMode       string
	PRUpdateMode string
	LogLevel     string
	// MaxPractices limits how many new practices are processed per run; 0 = unlimited.
	MaxPractices int
	// PRChecksWaitSeconds is how long to wait for the previous PR's CI before the next practice (0 disables wait).
	PRChecksWaitSeconds int
	// PRChecksPollSeconds is the polling interval while waiting for CI.
	PRChecksPollSeconds int

	SyncedStrategy string
	Granularity    string

	Mapping MappingConfig
}

type MappingConfig struct {
	Defaults        MappingDefaults   `yaml:"defaults"`
	Rules           []MappingRule     `yaml:"rules"`
	ServiceAliases  map[string]string `yaml:"service_aliases"`
	PracticeAliases map[string]string `yaml:"practice_aliases"`
}

type MappingDefaults struct {
	SkillID           string `yaml:"skill_id"`
	Template          string `yaml:"template"`
	TargetPathPattern string `yaml:"target_path_pattern"`
	Action            string `yaml:"action"`
}

type MappingRule struct {
	Match             string `yaml:"match"`
	SkillID           string `yaml:"skill_id"`
	Template          string `yaml:"template"`
	TargetPathPattern string `yaml:"target_path_pattern"`
}

type fileConfig struct {
	Repos struct {
		B struct {
			Repo          string   `yaml:"repo"`
			ExamplesPath  string   `yaml:"examples_path"`
			DefaultBranch string   `yaml:"default_branch"`
			IgnoreNames   []string `yaml:"ignore_names"`
		} `yaml:"b"`
		C struct {
			Repo           string   `yaml:"repo"`
			DocsRoot       string   `yaml:"docs_root"`
			DefaultBranch  string   `yaml:"default_branch"`
			SyncedManifest string   `yaml:"synced_manifest"`
			PathAllowlist  []string `yaml:"path_allowlist"`
		} `yaml:"c"`
	} `yaml:"repos"`
	AI struct {
		BaseURL            string `yaml:"base_url"`
		Model              string `yaml:"model"`
		TimeoutSeconds     int    `yaml:"timeout_seconds"`
		MaxRetries         int    `yaml:"max_retries"`
		MaxTokens          int    `yaml:"max_tokens"`
		ResponseFormatJSON bool   `yaml:"response_format_json"`
		MaxContextChars    int    `yaml:"max_context_chars"`
	} `yaml:"ai"`
	Skill struct {
		ID   string `yaml:"id"`
		Root string `yaml:"root"`
	} `yaml:"skill"`
	Runtime struct {
		DryRun       bool   `yaml:"dry_run"`
		WorkDir      string `yaml:"work_dir"`
		StatePath    string `yaml:"state_path"`
		PRMode       string `yaml:"pr_mode"`
		PRUpdateMode string `yaml:"pr_update_mode"`
		LogLevel     string `yaml:"log_level"`
		MaxPractices int    `yaml:"max_practices"`
	} `yaml:"runtime"`
	Detection struct {
		Granularity    string `yaml:"granularity"`
		SyncedStrategy string `yaml:"synced_strategy"`
	} `yaml:"detection"`
}

// FindRepoRoot walks up from cwd looking for go.mod or configs/default_config.yaml.
func FindRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if fileExists(filepath.Join(dir, "go.mod")) || fileExists(filepath.Join(dir, "configs", "default_config.yaml")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd, nil
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Load reads .env (optional), YAML defaults, then applies environment overrides.
func Load() (*Settings, error) {
	root, err := FindRepoRoot()
	if err != nil {
		return nil, err
	}
	_ = godotenv.Load(filepath.Join(root, ".env"))

	s := &Settings{
		RepoRoot: root,
		// BRepo / CRepo 无内置默认值，必须通过环境变量 B_REPO / C_REPO 提供。
		BExamplesPath:      "examples",
		BDefaultBranch:     "master",
		IgnoreNames:        []string{"README.md", "README", ".gitkeep"},
		CDocsRoot:          "docs/zh-cn/best-practices",
		CDefaultBranch:     "master",
		CSyncedManifest:    "synced-practices.json",
		PathAllowlist:      []string{"docs/zh-cn/", "docs/en-us/"},
		AIBaseURL:          "https://api.deepseek.com",
		AIModel:            "deepseek-chat",
		AITimeoutSeconds:   300,
		AIMaxRetries:       2,
		AIMaxTokens:        300000,
		MaxContextChars:    48000,
		ResponseFormatJSON: true,
		SkillID:            "best-practice-doc",
		SkillRoot:          "skills",
		StatePath:          "state/state.json",
		WorkDir:            ".work",
		PRMode:              "one_per_practice",
		PRUpdateMode:        "skip",
		LogLevel:            "INFO",
		PRChecksWaitSeconds: 120,
		PRChecksPollSeconds: 10,
		SyncedStrategy:      "hybrid",
		Granularity:         "nested_directory",
	}

	cfgPath := filepath.Join(root, "configs", "default_config.yaml")
	if data, err := os.ReadFile(cfgPath); err == nil {
		var fc fileConfig
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return nil, fmt.Errorf("parse %s: %w", cfgPath, err)
		}
		applyFileConfig(s, &fc)
	}

	mapPath := filepath.Join(root, "configs", "practice_mapping.yaml")
	if data, err := os.ReadFile(mapPath); err == nil {
		var mc MappingConfig
		if err := yaml.Unmarshal(data, &mc); err != nil {
			return nil, fmt.Errorf("parse %s: %w", mapPath, err)
		}
		s.Mapping = mc
	}

	applyEnv(s)
	return s, nil
}

func applyFileConfig(s *Settings, fc *fileConfig) {
	// repos.b.repo / repos.c.repo 已废弃：B_REPO / C_REPO 必须由环境变量提供，YAML 中的值忽略。
	if fc.Repos.B.ExamplesPath != "" {
		s.BExamplesPath = fc.Repos.B.ExamplesPath
	}
	if fc.Repos.B.DefaultBranch != "" {
		s.BDefaultBranch = fc.Repos.B.DefaultBranch
	}
	if len(fc.Repos.B.IgnoreNames) > 0 {
		s.IgnoreNames = fc.Repos.B.IgnoreNames
	}
	if fc.Repos.C.DocsRoot != "" {
		s.CDocsRoot = fc.Repos.C.DocsRoot
	}
	if fc.Repos.C.DefaultBranch != "" {
		s.CDefaultBranch = fc.Repos.C.DefaultBranch
	}
	if fc.Repos.C.SyncedManifest != "" {
		s.CSyncedManifest = fc.Repos.C.SyncedManifest
	}
	if len(fc.Repos.C.PathAllowlist) > 0 {
		s.PathAllowlist = fc.Repos.C.PathAllowlist
	}
	if fc.AI.BaseURL != "" {
		s.AIBaseURL = fc.AI.BaseURL
	}
	if fc.AI.Model != "" {
		s.AIModel = fc.AI.Model
	}
	if fc.AI.TimeoutSeconds > 0 {
		s.AITimeoutSeconds = fc.AI.TimeoutSeconds
	}
	if fc.AI.MaxRetries > 0 {
		s.AIMaxRetries = fc.AI.MaxRetries
	}
	if fc.AI.MaxTokens > 0 {
		s.AIMaxTokens = fc.AI.MaxTokens
	}
	if fc.AI.MaxContextChars > 0 {
		s.MaxContextChars = fc.AI.MaxContextChars
	}
	s.ResponseFormatJSON = fc.AI.ResponseFormatJSON || s.ResponseFormatJSON
	if fc.Skill.ID != "" {
		s.SkillID = fc.Skill.ID
	}
	if fc.Skill.Root != "" {
		s.SkillRoot = fc.Skill.Root
	}
	s.DryRun = fc.Runtime.DryRun
	if fc.Runtime.WorkDir != "" {
		s.WorkDir = fc.Runtime.WorkDir
	}
	if fc.Runtime.StatePath != "" {
		s.StatePath = fc.Runtime.StatePath
	}
	if fc.Runtime.PRMode != "" {
		s.PRMode = fc.Runtime.PRMode
	}
	if fc.Runtime.PRUpdateMode != "" {
		s.PRUpdateMode = fc.Runtime.PRUpdateMode
	}
	if fc.Runtime.LogLevel != "" {
		s.LogLevel = fc.Runtime.LogLevel
	}
	if fc.Runtime.MaxPractices > 0 {
		s.MaxPractices = fc.Runtime.MaxPractices
	}
	if fc.Detection.SyncedStrategy != "" {
		s.SyncedStrategy = fc.Detection.SyncedStrategy
	}
	if fc.Detection.Granularity != "" {
		s.Granularity = fc.Detection.Granularity
	}
}

func applyEnv(s *Settings) {
	set := func(key string, dst *string) {
		if v := os.Getenv(key); v != "" {
			*dst = v
		}
	}
	set("B_REPO", &s.BRepo)
	set("B_REPO_TOKEN", &s.BRepoToken)
	set("B_EXAMPLES_PATH", &s.BExamplesPath)
	set("B_DEFAULT_BRANCH", &s.BDefaultBranch)
	set("C_REPO", &s.CRepo)
	set("C_REPO_TOKEN", &s.CRepoToken)
	set("C_DOCS_ROOT", &s.CDocsRoot)
	set("C_DEFAULT_BRANCH", &s.CDefaultBranch)
	set("C_SYNCED_MANIFEST", &s.CSyncedManifest)
	set("AI_BASE_URL", &s.AIBaseURL)
	set("AI_API_KEY", &s.AIAPIKey)
	set("AI_MODEL", &s.AIModel)
	set("SKILL_ID", &s.SkillID)
	set("STATE_PATH", &s.StatePath)
	set("WORK_DIR", &s.WorkDir)
	set("PR_MODE", &s.PRMode)
	set("PR_UPDATE_MODE", &s.PRUpdateMode)
	set("LOG_LEVEL", &s.LogLevel)

	setIntEnv := func(key string, dst *int) {
		v := os.Getenv(key)
		if v == "" {
			return
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: ignore invalid %s=%q (keep %d)\n", key, v, *dst)
			return
		}
		*dst = n
	}
	setIntEnv("AI_TIMEOUT_SECONDS", &s.AITimeoutSeconds)
	setIntEnv("AI_MAX_RETRIES", &s.AIMaxRetries)
	setIntEnv("AI_MAX_TOKENS", &s.AIMaxTokens)
	setIntEnv("MAX_PRACTICES", &s.MaxPractices)
	setIntEnv("PR_CHECKS_WAIT_SECONDS", &s.PRChecksWaitSeconds)
	setIntEnv("PR_CHECKS_POLL_SECONDS", &s.PRChecksPollSeconds)

	if v := os.Getenv("DRY_RUN"); v != "" {
		s.DryRun = parseBool(v)
	}
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

var (
	ownerNameRe = regexp.MustCompile(`^[\w.-]+/[\w.-]+$`)
	githubURLRe = regexp.MustCompile(`(?i)(?:https?://)?github\.com[:/]([\w.-]+)/([\w.-]+?)(?:\.git)?/?$`)
)

// ValidateRepoRef accepts owner/name or a github.com URL.
func ValidateRepoRef(repo string) error {
	value := strings.TrimSpace(repo)
	if value == "" {
		return fmt.Errorf("empty repository reference")
	}
	if ownerNameRe.MatchString(value) {
		return nil
	}
	if githubURLRe.MatchString(value) {
		return nil
	}
	return fmt.Errorf("invalid repository %q (want owner/name or github.com URL)", repo)
}

func (s *Settings) RequireRepos() error {
	var missing []string
	// 强制要求环境变量；不允许仅依赖 YAML / 代码默认值。
	if strings.TrimSpace(os.Getenv("B_REPO")) == "" || strings.TrimSpace(s.BRepo) == "" {
		missing = append(missing, "B_REPO")
	}
	if strings.TrimSpace(os.Getenv("C_REPO")) == "" || strings.TrimSpace(s.CRepo) == "" {
		missing = append(missing, "C_REPO")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s (owner/name, no built-in default)", strings.Join(missing, ", "))
	}
	if err := ValidateRepoRef(s.BRepo); err != nil {
		return fmt.Errorf("B_REPO: %w", err)
	}
	if err := ValidateRepoRef(s.CRepo); err != nil {
		return fmt.Errorf("C_REPO: %w", err)
	}
	return nil
}

// RequireCRepoTokenForPush requires C_REPO_TOKEN when not in dry-run (push / open PR).
func (s *Settings) RequireCRepoTokenForPush() error {
	if strings.TrimSpace(s.CRepoToken) == "" {
		return fmt.Errorf("missing required environment variable: C_REPO_TOKEN (required when DRY_RUN is false)")
	}
	return nil
}

func (s *Settings) HasAIAPIKey() bool {
	return strings.TrimSpace(s.AIAPIKey) != ""
}

func (s *Settings) AbsoluteStatePath() string {
	if filepath.IsAbs(s.StatePath) {
		return s.StatePath
	}
	return filepath.Join(s.RepoRoot, s.StatePath)
}

func (s *Settings) AbsoluteWorkDir() string {
	if filepath.IsAbs(s.WorkDir) {
		return s.WorkDir
	}
	return filepath.Join(s.RepoRoot, s.WorkDir)
}

func (s *Settings) SkillDir() string {
	return filepath.Join(s.RepoRoot, s.SkillRoot, s.SkillID)
}

func (s *Settings) TemplatesDir() string {
	return filepath.Join(s.RepoRoot, "templates")
}
