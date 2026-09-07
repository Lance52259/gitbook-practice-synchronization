# gitbook-practice-synchronization

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![CI](https://github.com/chnsz/gitbook-practice-synchronization/actions/workflows/test-workflow.yml/badge.svg)](https://github.com/chnsz/gitbook-practice-synchronization/actions/workflows/test-workflow.yml)

从 **B 仓**（环境变量 `B_REPO`）的 `examples/` 自动发现新增最佳实践，按 Skill 约束调用 DeepSeek 生成 **中英双语文档**，并向 **C 仓**（环境变量 `C_REPO`）提交 Pull Request。

---

## 目录

- [背景](#背景)
- [功能特性](#功能特性)
- [工作原理](#工作原理)
- [环境要求](#环境要求)
- [快速开始](#快速开始)
- [配置](#配置)
- [CLI 用法](#cli-用法)
- [GitHub Actions](#github-actions)
- [项目结构](#项目结构)
- [开发](#开发)
- [许可证](#许可证)

---

## 背景

华为云最佳实践文档仓（C，由 `C_REPO` 指定）需要与 Provider examples 源仓（B，由 `B_REPO` 指定）保持同步。手工对照成本高、易漏改导航（尤其是 `SUMMARY.md`）。

**gitbook-practice-synchronization** 把「探测 → 生成 → 开 PR」做成可重复流水线：本仓库负责编排与 Skill；源仓提供 examples；文档仓接收变更。

| 角色 | 仓库 | 职责 |
|------|------|------|
| **A** | 本仓库（**gitbook-practice-synchronization**） | CLI、Skill、映射、Actions |
| **B** | `$B_REPO` 指定的源仓库，如：[huaweicloud/terraform-provider-huaweicloud](https://github.com/huaweicloud/terraform-provider-huaweicloud) | `examples/` 最佳实践源 |
| **C** | `$C_REPO` 指定的目标推送仓库，如：[chnsz/hcbp-demo](https://github.com/chnsz/hcbp-demo) | 中英文档与 PR 目标 |

---

## 功能特性

- **增量探测**：对比 B 仓 `examples/` 与 C 仓已有文档（含服务别名、连字符/下划线模糊匹配）
- **Open PR 跳过（按服务）**：扫描 C 仓本工具创建的 `gitbook-practice-synchronization/...` 分支上的 open PR，解析标题中的 `docs({service})`；**该服务下所有未对接实践一律跳过**，直到 PR 合入后的下一次扫描。同一次扫描内每个服务最多处理 **1** 条实践，避免并行污染 `index.md` / `SUMMARY.md`
- **干净基线生成**：每条实践 Generate / Apply 前将 C 工作树 `reset --hard` 到 `origin/$C_DEFAULT_BRANCH`，避免同一次 run 中上一条 PR 的导航残留写进下一条（跨服务污染）
- **中英双语生成**：按 Skill 顺序产出 `docs/zh-cn/` 与 `docs/en-us/` 正文
- **安全导航补丁**：`SUMMARY.md` / `index.md` / `README.md` 仅定点插入；英文侧先按字母序定目录，中文侧跟随；禁止整文件重写
- **一实践一 PR**：提交信息与 PR 标题统一为 `docs({service}): support new best practice for {title}`
- **本地 Dry-run**：不 push、不开真实 PR，便于联调
- **定时 + 手动**：GitHub Actions 支持 schedule 与 `workflow_dispatch`

---

## 工作原理

```text
┌─────────────┐     clone/diff      ┌──────────────────────┐
│ B: examples │ ─────────────────►  │ detect new practices │
└─────────────┘                     └──────────┬───────────┘
                                               │
                                               ▼
                                    ┌──────────────────────┐
                                    │ Skill + DeepSeek     │
                                    │ → zh/en markdown     │
                                    └──────────┬───────────┘
                                               │
                                               ▼
┌─────────────┐     branch + PR     ┌──────────────────────┐
│ C: docs     │ ◄─────────────────  | nav patch + gitops   │
└─────────────┘                     └──────────────────────┘
```

1. 拉取 B / C 工作树  
2. 枚举 examples，过滤已对接文档；按 open PR **所属服务**跳过，且每服务每轮最多 1 条
3. 按 `MAX_PRACTICES` 限流后调用 AI 生成（每条前复位 C 工作树到干净 base）  
4. 编排层补丁中英导航文件  
5. 推送分支并创建 PR（非 dry-run）

---

## 环境要求

- Go **1.22+**
- Git
- DeepSeek API Key（生成阶段）
- 对 C 仓有写权限的 GitHub Token（`Contents` + `Pull requests`；非 dry-run）

---

## 快速开始

```bash
git clone https://github.com/chnsz/gitbook-practice-synchronization.git
cd gitbook-practice-synchronization

cp .env.example .env
# 编辑 .env：必填 B_REPO、C_REPO、AI_API_KEY；正式推 PR 还需 C_REPO_TOKEN

make build
make detect          # 仅探测，打印 JSON
make dry-run         # 全流程 dry-run（不 push / 不开真实 PR）
```

正式执行（会写 C 仓）：

```bash
make run
# 或
./bin/gitbook-practice-synchronization run
./bin/gitbook-practice-synchronization run --practice examples/ecs/basic
```

---

## 配置

环境变量优先于 `configs/default_config.yaml`。完整示例见 [`.env.example`](./.env.example)。

本地：写入 `.env` 即可。  
GitHub Actions：本仓库工作流使用 `environment: Development`，请在 **Settings → Environments → Development** 中配置；工作流通过 `secrets.*` / `vars.*` 注入（见下表 **Actions 位置**）。

| Actions 位置 | 含义 |
|--------------|------|
| **Secret** | Environment / Repo **Secrets**；工作流用 `secrets.NAME`（适合 token、API Key；勿把普通开关误放这里） |
| **Variable** | Environment / Repo **Variables**；工作流用 `vars.NAME`（适合非敏感配置，如分支名、条数上限） |
| **Secret 或 Variable** | 工作流优先读 Secret，缺省再用 Variable / 字面默认值（见各行说明） |
| **仅本地** | Actions 未引用；只在 `.env` / YAML 中使用 |

### B 仓

| 变量 | 默认 | Actions 位置 | 说明 |
|------|------|--------------|------|
| `B_REPO` | —（**必填**） | **Secret**（`secrets.B_REPO`） | 源仓库 `owner/name`，无内置默认；如 `huaweicloud/terraform-provider-huaweicloud` |
| `B_REPO_TOKEN` | _(空)_ | **Secret** | 读 B 仓；公开仓可省略 |
| `B_EXAMPLES_PATH` | `examples` | **仅本地**（Actions 未单独注入） | examples 根路径 |
| `B_DEFAULT_BRANCH` | `master` | **Variable**（`vars.B_DEFAULT_BRANCH`） | B 仓分支 |

### C 仓

| 变量 | 默认 | Actions 位置 | 说明 |
|------|------|--------------|------|
| `C_REPO` | —（**必填**） | **Secret**（`secrets.C_REPO`） | 文档 / PR 目标仓 `owner/name`，无内置默认；如 `chnsz/hcbp-demo` |
| `C_REPO_TOKEN` | _(空)_ | **Secret**（必填，非 dry-run） | 写分支 + 开 PR |
| `C_DOCS_ROOT` | `docs/zh-cn/best-practices` | **仅本地** | 中文文档根（探测用） |
| `C_DEFAULT_BRANCH` | `master` | **Variable**（`vars.C_DEFAULT_BRANCH`） | PR base |
| `C_SYNCED_MANIFEST` | `synced-practices.json` | **仅本地** | 可选已对接清单 |

### AI 与运行时

| 变量 | 默认 | Actions 位置 | 说明 |
|------|------|--------------|------|
| `AI_API_KEY` | — | **Secret**（生成必填；缺省时 `run`/`generate` 醒目跳过并 **exit 0**） | DeepSeek API Key |
| `AI_BASE_URL` | `https://api.deepseek.com` | **Variable**（`vars.AI_BASE_URL`） | API 地址 |
| `AI_MODEL` | `deepseek-chat` | **Variable**（`vars.AI_MODEL`） | 模型名 |
| `AI_MAX_TOKENS` | `300000` | **Variable**（`vars.AI_MAX_TOKENS`） | 单次完成最大 token；双语文档建议拉高。DeepSeek V4 输出硬上限约 384000 |
| `AI_TIMEOUT_SECONDS` | `300` | **Variable**（`vars.AI_TIMEOUT_SECONDS`） | 请求超时（秒）；拉高 max_tokens 后建议同步加大 |
| `DRY_RUN` | `false` | workflow 输入 / 本地 `.env` | `true` 时不 push / 不开真实 PR |
| `MAX_PRACTICES` | 本地 `0`（不限制）；Actions 未配置时回退 `1` | **Variable**（`vars.MAX_PRACTICES`） | 过滤后单次最多处理条数（先按服务跳过 open PR，再每服务留 1 条，最后才截断）。**必须配在 Variables，不要放进 Secrets**；放错则 `vars` 读不到，会一直用默认 `1` |
| `SKILL_ID` | `best-practice-doc` | **Variable**（`vars.SKILL_ID`） | Skill 目录名 |

路径映射（B `examples` → C 服务目录 / 文件名）见 [`configs/practice_mapping.yaml`](./configs/practice_mapping.yaml)。  
文档生成规则见 [`skills/best-practice-doc/SKILL.md`](./skills/best-practice-doc/SKILL.md)。

---

## CLI 用法

```bash
gitbook-practice-synchronization detect [--out new.json] [--no-refresh]
gitbook-practice-synchronization generate [--practice ID] [--practices-file FILE] [--dry-run]
gitbook-practice-synchronization run [--practice ID] [--dry-run=true|false] [--no-refresh]
```

| 命令 | 说明 |
|------|------|
| `detect` | 输出 `new_practices` / `synced_ids` / `skipped_open_pr` |
| `generate` | 仅生成（默认偏 dry-run 行为，见 flag） |
| `run` | 探测 → 生成 → 推送 → 开 PR |

构建产物：`make build` → `./bin/gitbook-practice-synchronization`。

---

## GitHub Actions

工作流：[`.github/workflows/monitor-and-generate.yml`](./.github/workflows/monitor-and-generate.yml)

| 触发 | 说明 |
|------|------|
| **schedule** | 每天 **东八区 02:00**（`cron: 0 18 * * *` UTC） |
| **workflow_dispatch** | 可选手动指定 `practice`、勾选 `dry_run` |

Secrets / Variables 建议放在 **Environment `Development`**（工作流已声明 `environment: Development`）：

| 类型 | 配置项 |
|------|--------|
| **Secrets（必填）** | `B_REPO`、`C_REPO`、`C_REPO_TOKEN` |
| **Secrets（生成用）** | `AI_API_KEY`（未配置时流水线跳过生成并以成功结束） |
| **Secrets（可选）** | `B_REPO_TOKEN` |
| **Variables（可选）** | `MAX_PRACTICES`、`B_DEFAULT_BRANCH`、`C_DEFAULT_BRANCH`、`AI_BASE_URL`、`AI_MODEL`、`AI_MAX_TOKENS`、`AI_TIMEOUT_SECONDS`、`SKILL_ID` |

`MAX_PRACTICES` 等非敏感项请放 **Variables**；若放进 Secrets，工作流的 `vars.MAX_PRACTICES` 读不到，会回退为 `1`。

单元测试工作流：[`test-workflow.yml`](./.github/workflows/test-workflow.yml)（push / PR）。

---

## 项目结构

```text
gitbook-practice-synchronization/
├── cmd/
│   └── gitbook-practice-synchronization/   # CLI 入口
├── internal/
│   ├── monitor/                            # clone、探测、状态
│   ├── mapping/                            # B → C 服务/实践映射
│   ├── ai/                                 # Skill、Prompt、生成
│   │   └── provider/                       # DeepSeek OpenAI Compatible
│   ├── nav/                                # 中英 SUMMARY / index / README 手术式补丁
│   ├── gitops/                             # commit、push、PR
│   └── config/                             # .env + YAML
├── configs/                                # default_config.yaml、practice_mapping.yaml
├── skills/
│   └── best-practice-doc/                  # 文档生成 Skill
├── templates/                              # 正文 / 分类 / PR body 模板
└── .github/
    ├── actions/gitbook-practice-synchronization-action/
    └── workflows/                          # 定时生成 + 单元测试
```

---

## 开发

```bash
make tidy
make test
make build
```

- 测试与源码同目录：`*_test.go`
- 本地联调建议：`MAX_PRACTICES=1` + `DRY_RUN=true`（同服务多实践时，未合入的 open PR 会挡住该服务其余条目）
- 生成规范变更请同步更新 `skills/best-practice-doc/SKILL.md`

---

## 许可证

本项目基于 [MIT License](./LICENSE) 发布。
