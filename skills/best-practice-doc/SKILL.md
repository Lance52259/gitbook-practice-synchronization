---
name: best-practice-doc
description: 从 B 仓 examples 生成中英双语文档并 PR 到 C 仓（仓库由环境变量 / 配置指定，勿写死）
version: "0.3.9"
# B/C 具体仓库不在此写死。运行时由必填环境变量 B_REPO / C_REPO（及 .env）注入。
role_b: source examples repo（B_REPO, B_EXAMPLES_PATH, B_DEFAULT_BRANCH）
role_c: docs + PR target repo（C_REPO, C_DOCS_ROOT, C_DEFAULT_BRANCH）
---

# Skill：华为云最佳实践文档生成（B → C）

## 何时使用

当 **B 仓库**（`B_REPO`，其 `examples/` 或 `B_EXAMPLES_PATH`）出现尚未对接至 **C 仓库**（`C_REPO`）的最佳实践时使用本 Skill。  
B/C 的 owner/name、默认分支均以**当前运行配置为准**，本文件不绑定某一固定 GitHub 仓库。

## 目标

依据 B 仓实践目录中的 Terraform HCL，按 C 仓模板与导航约定，生成 **中文 + 英文** Markdown 变更，并由 gitbook-practice-synchronization **向 C 仓推送分支并创建 PR**（基于 `C_DEFAULT_BRANCH`，常见为 `master`）。

---

## 强制处理顺序（必须按 1 → 8 执行）

新增一条最佳实践时，**按下列顺序**生成/更新文件。  
**步骤 2–5 先定英文侧目录顺序（英文字母序）**；步骤 6–8 的中文导航必须 **跟随英文侧已确定的顺序**，不得按中文标题自行重排。

| 步骤 | 动作 | 路径 | 说明 |
|------|------|------|------|
| **1** | **create** | `docs/zh-cn/best-practices/{service}/{practice}.md` | 中文版最佳实践正文 |
| **2** | **create** | `docs/en-us/best-practices/{service}/{practice}.md` | 英文版最佳实践正文（与中文同一 `{service}/{practice}`） |
| **3** | **create** 或 **update** | `docs/en-us/best-practices/{service}/index.md` | 英文分类页：**新建**须按「分类 index.md 规范」写满（对齐 Anti-DDoS，禁止精简版）；已存在则只在 Best Practices List 按 practice 文件名字母序插入本条（含一句话说明） |
| **4** | **update** | `docs/en-us/best-practices/README.md` | 英文文档导航：按服务目录名字母序插入；**标题与简介必须从英文 `index.md` 截取**（规则见「README 导航条目规范」）。禁止占位句。新服务必做；已有服务仅新增实践时可跳过 |
| **5** | **update** | `docs/en-us/SUMMARY.md` | 英文 TOC：**一定存在**，只定点插入。已有服务 → 在该服务块内按 practice 文件名字母序插入实践链接；**新增服务** → 再按服务目录名字母序插入「服务节点 + Introduction + 本实践」 |
| **6** | **create** 或 **update** | `docs/zh-cn/best-practices/{service}/index.md` | 中文分类页：**新建**须按「分类 index.md 规范」写满（与英文同等篇幅）；已存在则只追加列表项。**列表顺序与英文 index 一致** |
| **7** | **update** | `docs/zh-cn/best-practices/README.md` | 中文文档导航：顺序跟随英文；**标题与简介必须从中文 `index.md` 截取**（规则见下节「README 导航条目规范」）。禁止占位句。新服务必做；已有服务仅新增实践时可跳过 |
| **8** | **update** | `docs/zh-cn/SUMMARY.md` | 中文 TOC：**一定存在**，只定点插入。结构/相对位置与英文 SUMMARY **对齐**（同一 `{service}/{practice}` 链接路径；**新增服务**时同步插入服务块） |

### 顺序与排序原则

1. **英文先定序**：index / README / SUMMARY 的插入位置一律按 **英文路径名（`{service}`、`{practice}.md`）字母序** 计算。  
2. **中文跟随**：中文 index 列表、README 服务块、SUMMARY 服务块与实践行的相对顺序，与英文侧保持一致；不要用中文标题拼音或汉字顺序重排。  
3. **双语路径对称**：中英文使用相同的 `{service}`、`{practice}` 文件名；仅标题与正文语言不同。

### 命名约定

- `{service}`：华为云服务简称，**全小写**（如 `ecs`、`anti-ddos`），与 C 仓目录一致；不等于 B 仓 `examples/` 照抄（见 `configs/practice_mapping.yaml` 的 `service_aliases`）。
- `{practice}`：实践文件名（无 `.md`），中英共用；优先下划线风格，可用 `practice_aliases` 覆盖。

---

## 导航文件硬性规则（中英 SUMMARY / index / README）

> 历史失败：把 `SUMMARY.md` 整文件重写成短目录。中英两侧均 **严禁** 再次发生。

1. `docs/en-us/SUMMARY.md` 与 `docs/zh-cn/SUMMARY.md` **一定存在**：只能基于当前全文 **定点插入**，禁止整文件重写；禁止改标题（英文保持 `# Summary`，中文保持 `# Summary`）。
2. **index.md**：服务目录已存在 → **update** 列表项；服务目录首次创建 → **create** 全文（不要 update 不存在的文件）。**新建全文必须达到 Anti-DDoS 同等结构与篇幅**（见「分类 index.md 规范」），禁止只写一句话简介 + 裸链接列表。
3. **README.md（中英均适用，新增服务时）**：必须按「README 导航条目规范」从对应语言的 `index.md` 截取标题与简介，**禁止占位描述**。
4. 所有导航 `update` 必须是「基线全文 + 最小插入」；非空行数不得明显少于基线。
5. 优先由 gitbook-practice-synchronization 编排层对导航做确定性补丁；模型至少输出步骤 1–2 的正文，新服务时输出中英 `index.md` 的 create（须符合「分类 index.md 规范」，含完整「What is / 什么是」多段介绍，供 README 截取）。

### README 导航条目规范（`docs/{zh-cn|en-us}/best-practices/README.md`）

每个服务在「文档导航 / Documentation Navigation」下为一块，格式：

```markdown
### [{标题}]({service}/index.md)

{简介段落}
```

#### 英文

- **标题**：`{Name from ## What is …} Best Practices`  
  例：`### [Anti-DDoS Best Practices](anti-ddos/index.md)`、`### [Application Operations Management (AOM) Best Practices](aom/index.md)`
- **简介**：取英文 `index.md` 中 `## What is …` 下**第一段全文**（编排层截取；仅当整段异常长时才按句边界裁到长度上限，**禁止**因首句很短就丢掉整段）。正确示例须含产品能力描述，而非一句 `… provided by Huawei Cloud.`
- **禁止**：`AAD Terraform best practices.`、`XXX related best practices.`、仅首句残缺简介等占位/过度截断

#### 中文（与英文同一问题，必须同等遵守）

- **标题**：`{名称 from ## 什么是…}最佳实践`（与现网 Anti-DDoS / AOM 条目一致）  
  例：  
  - `### [Anti-DDoS最佳实践](anti-ddos/index.md)`  
  - `### [应用运维管理（AOM）最佳实践](aom/index.md)`  
  - `### [DDoS高防（AAD）最佳实践](aad/index.md)`
- **简介**：取中文 `index.md` 中 `## 什么是…` 下**第一段全文**（过长才按句裁剪），须是产品介绍，不是实践列表说明  
  **正确示例（对齐 Anti-DDoS / AAD）：**  
  `DDoS高防（Advanced Anti-DDoS，AAD）是华为云提供的专业DDoS防护服务，旨在保护互联网服务器和应用免受分布式拒绝服务（DDoS）攻击及其他恶意流量的影响。AAD提供全面的防护能力，包括DDoS流量清洗、CC（Challenge Collapsar）攻击防护和智能流量分析，确保在线服务的可用性和稳定性。`  
  **错误示例（禁止）：**  
  - `AAD 相关 Terraform 最佳实践。`  
  - `DDoS高防 Terraform 最佳实践。`  
  - `介绍如何使用 Terraform 完成本实践。`
- 中英 README 简介应语义对齐、篇幅相当；**不得**中文整段、英文只剩首句（曾出现过英文过度截断问题）。

---

## 不要做的事

- 不要只生成中文、省略英文（步骤 2–5 为必做链路）。
- 不要先改中文导航再改英文（会破坏「英文定序、中文跟随」）。
- 不要编造源脚本中不存在的 resource / data source / 参数。
- 不要自由新增模板未定义的一级/二级标题。
- 不要把元注释写进最终 Markdown；敏感信息用占位符。
- 链接使用半角 `()`；每个 `.md` 文件末尾保留一个空行。
- 不要用精简版目录替换完整 `SUMMARY.md`。
- 英文 `## Reference Information` 中源码链接锚文本必须为  
  `Best Practice Source Code Reference For {ServiceName} {PracticeObject}`，  
  例如 `Best Practice Source Code Reference For AAD Black/White Lists`；  
  **禁止**写成 `{title} Best Practice Source Code Reference`，也**禁止**把正文 H1 的 `Deploy` 写进锚文本（错误：`… For Deploy Black/White Lists`）。
- 中文 / 英文 `best-practices/README.md` 新增服务条目的简介必须从对应 `index.md` 的「什么是 / What is」首段截取，  
  禁止 `AAD 相关 Terraform 最佳实践。` / `AAD Terraform best practices.` 等占位句。
- 新建服务的 `index.md` 禁止精简版（单段 What is、一句 Overview、无列表导语/无条目说明、参考资料写成 Provider 文档）。
- 正文 H1 / index 列表标题禁止写成 `AAD黑白名单最佳实践`、`AAD Black/White Lists`（缺 Deploy/部署、或中文带「最佳实践」后缀）；须对齐 `部署…` / `Deploy …`。
- 无 data source 时禁止导语写「资源和数据源」且禁止空的 `### 数据源` 小节。
- tfvars 步骤标题必须含「（可选）」/ `(Optional)`。
- 同一 `variable "name"` 不得在多个操作步骤中重复声明（后步只引用 `var.name`）；生成后须自检去重。

---

## 路径推断

典型源路径：`examples/{service}/{practice}/`。

| 输入 | 输出 |
|------|------|
| `practice_id` = `examples/ecs/basic` | 中文正文 → `docs/zh-cn/best-practices/ecs/{practice}.md`；英文正文 → `docs/en-us/best-practices/ecs/{practice}.md`（`{practice}` 以映射/C 仓约定为准） |
| 服务是否首次 | 分别扫描 `docs/zh-cn/best-practices/{service}/` 与 `docs/en-us/best-practices/{service}/`（通常应同步存在；以实际为准） |

---

## 中文正文结构（步骤 1，固定标题顺序）

对齐 `docs/zh-cn/best-practices/anti-ddos/basic.md`、`ecs/simple_instance.md` 与 `templates/best_practice_template.md`。

### 标题与命名（与现网一致）

- **H1**：`# 部署{场景简述}`  
  - **正确：** `# 部署基础防护`、`# 部署基础实例`、`# 部署黑白名单`  
  - **错误（禁止，PR #3 曾出现）：** `# AAD黑白名单最佳实践`（勿以服务缩写开头；**H1 不要以「最佳实践」结尾**）
- **index 列表 / SUMMARY 实践行标题**：与正文 **H1 全文一致**（含「部署」；不含「最佳实践」后缀）
- **应用场景**第二段起句：用 `本最佳实践将介绍如何使用Terraform…`（不要写成「本实践将介绍」）

### 章节顺序

1. `# 部署{场景简述}`
2. `## 应用场景`
3. `## 相关资源/数据源`
4. `## 操作步骤`
5. `## 参考信息`

### 相关资源/数据源（按源码实际情况二选一）

**有 data source 时**（对齐 ECS）：

- 导语固定：`本最佳实践涉及以下主要资源和数据源：`
- 必须含小节：`### 数据源` → `### 资源` → `### 资源/数据源依赖关系`

**无 data source 时**（对齐 Anti-DDoS；AAD 黑白名单属此类）：

- 导语固定：`本最佳实践涉及以下主要资源：`（**禁止**写「资源和数据源」）
- **不要**输出空的 `### 数据源`
- 只保留：`### 资源` → `### 资源/数据源依赖关系`（依赖树标题仍用此固定名）

资源链接显示名用产品中文名 + resource type，如 `[DDoS高防实例（huaweicloud_aad_instance）](...)`。

### 操作步骤硬性要求

1. **脚本准备**（固定文案，含 `prepare_before_deploy.md` 链接）
2. **按源码顺序**逐步创建 resource/data；**每个步骤的 HCL 块内须内联「本步首次引入」的 `variable` 声明**（对齐 Anti-DDoS/ECS），再写 `resource`/`data`。即使源码把变量放在 `variables.tf`，文档步骤中仍应展开内联（object 类型变量可整段内联一个 `variable "instance_config"`）。
3. **跨步骤变量去重（生成后自检，必做）**：全文所有操作步骤 HCL 中，同一 `variable "name"` **只允许出现一次**。  
   - 在**第一次**用到该变量的步骤中声明；后续步骤（如黑名单步骤 3、白名单步骤 4 都依赖 `instance_id`）**只引用** `var.xxx`，**禁止再次粘贴**整段 `variable "xxx" { ... }`。  
   - 生成中英文正文后必须自检：统计每个 `variable "…"` 出现次数；若 >1，删掉后出现的重复声明，仅保留首次。  
   - **错误示例（禁止）：** 步骤 2 已声明 `variable "instance_id"`，步骤 3、4 的 HCL 里再次完整声明 `variable "instance_id"`。  
   - **正确做法：** 步骤 2 声明一次；步骤 3/4 的 HCL 仅含 `resource`（及本步**新**引入的变量，如 `blacklist_ips` / `whitelist_ips`）。
4. **参数说明**：对块内主要参数逐条说明；引用变量时尽量写清「通过引用输入变量 xxx 进行赋值」（对齐 Anti-DDoS）。仅说明本步 resource 参数；勿因去重而在后续步骤重复粘贴已在前步说明过的 variable 块。
5. **region 注释**（HCL 上方）：  
   - 资源/数据源声明了 `region` → `# 在指定region下…`  
   - **未声明 `region`** → 必须用完整句：`# 在指定region（region参数缺省时默认继承当前provider块中所指定的region）下…`  
   - **禁止**无 region 参数时只写短句 `# 在指定region下创建…`
6. **预设入参步骤标题必须为**：`### N. 预设资源部署所需的入参（可选）`（**「（可选）」不可省略**）
7. **最后一步**：`### N. 初始化并应用Terraform配置`（init / plan / apply / show）

### 参考信息

- 华为云对应产品文档 index
- 固定：`[华为云Provider文档](https://registry.terraform.io/providers/huaweicloud/huaweicloud/latest/docs)`
- 源码目录链接，锚文本：`[{服务或产品名}{场景简述}最佳实践源码参考](https://github.com/{B_REPO}/tree/{B_DEFAULT_BRANCH}/examples/{b_service}/{b_practice})`  
  - `{B_REPO}` / `{B_DEFAULT_BRANCH}` 取自当前运行配置（勿写死某一组织/仓库）  
  - **正确形态：** `[Anti-DDoS基础防护最佳实践源码参考](...)`、`[AAD黑白名单最佳实践源码参考](...)`  
  - **错误：** 英文锚文本、仅写 `源码参考`  
  - 注意：源码锚文本可含服务名 + 场景 +「最佳实践源码参考」；**不等于**把「最佳实践」写进正文 H1

---

## 英文正文结构（步骤 2，固定标题顺序）

对齐 `docs/en-us/best-practices/anti-ddos/basic.md`、`ecs/simple_instance.md`（与中文镜像）。

### Title and naming

- **H1**：`# Deploy {Scene}` — **must start with `Deploy`**  
  - **Correct:** `# Deploy Basic Protection`、`# Deploy Basic Instance`、`# Deploy Black/White Lists`  
  - **Wrong (forbidden):** `# AAD Black/White Lists`（missing `Deploy`）
- **index list / SUMMARY practice title**: **identical to H1**
- **Application Scenario** second paragraph lead-in: `This best practice will introduce how to use Terraform…`（**not** `This practice will introduce`）

### Section order

1. `# Deploy {Scene}`
2. `## Application Scenario`
3. `## Related Resources/Data Sources`
4. `## Operation Steps`
5. `## Reference Information`

### Related Resources/Data Sources

**When the source has data sources** (ECS-style):

- Lead-in: `This best practice involves the following main resources and data sources:`
- Subsections: `### Data Sources` → `### Resources` → `### Resource/Data Source Dependencies`

**When there is no data source** (Anti-DDoS / AAD-style):

- Lead-in: `This best practice involves the following main resources:`（**do not** say “resources and data sources”）
- **Omit** empty `### Data Sources`
- Keep: `### Resources` → `### Resource/Data Source Dependencies`

Use clear product display names, e.g. `[Advanced Anti-DDoS Instance (huaweicloud_aad_instance)](...)`.

### Operation Steps

1. Script Preparation (with link to `prepare_before_deploy.md`)
2. Per-resource steps: **inline `variable` blocks only for variables first introduced in that step**, then `resource`/`data` (same as Anti-DDoS/ECS docs)
3. **Deduplicate variables across steps (mandatory self-check after drafting):** each `variable "name"` may appear **once** in the whole practice doc. Declare it in the first step that needs it; later steps (e.g. blacklist step 3 and whitelist step 4 both using `instance_id`) must **only reference** `var.xxx` — **do not paste** the `variable` block again. If any name appears more than once, delete the later copies.
4. Region comment above HCL: if no `region` argument, use  
   `# Create … in the specified region (if the region parameter is omitted, it inherits the region specified in the current provider block)`  
   （勿在句末多余堆砌 `by default`）
5. **tfvars step title must be:**  
   `### N. Preset Input Parameters Required for Resource Deployment (Optional)`  
   （`Required` + `(Optional)` 均不可省；禁止写成 `Preset Input Parameters for Resource Deployment`）
6. Final step: `### N. Initialize and Apply Terraform Configuration`

### Reference Information

- Huawei Cloud product documentation index for the service
- Fixed: `[Huawei Cloud Provider Documentation](https://registry.terraform.io/providers/huaweicloud/huaweicloud/latest/docs)`
- Source link anchor (**exact pattern**):  
  `[Best Practice Source Code Reference For {ServiceName} {PracticeObject}](https://github.com/{B_REPO}/tree/{B_DEFAULT_BRANCH}/examples/{b_service}/{b_practice})`  
  - `{ServiceName}`：服务英文名或常用简称（如 `AAD`、`Anti-DDoS`、`ECS`）  
  - `{PracticeObject}`：最佳实践对象/场景名词短语，通常等于 H1 去掉前缀 `Deploy ` 之后的部分（如 H1=`Deploy Black/White Lists` → `Black/White Lists`）  
  - **完整锚文本 =** `Best Practice Source Code Reference For` + 空格 + `{ServiceName}` + 空格 + `{PracticeObject}`  
  - URL 中 `{B_REPO}` / 分支取自当前配置  
  - **Correct:**  
    `[Best Practice Source Code Reference For AAD Black/White Lists](...)`  
    `[Best Practice Source Code Reference For Anti-DDoS Basic Protection](...)`  
    `[Best Practice Source Code Reference For ECS Basic Instance](...)`  
  - **Wrong:**  
    `Best Practice Source Code Reference For Deploy Black/White Lists`（多了 H1 的 `Deploy`，缺服务名）  
    `AAD Black/White Lists Best Practice Source Code Reference`（语序颠倒）  
    `Best practice source code reference for ...`（大小写错误）

内容须与中文版同一套资源/参数/步骤，禁止中英不一致或英文臆造。源码 URL 使用 B 仓真实 `examples/` 路径（可能与 C 仓 `{service}/{practice}` 映射名不同）。

---
## 分类 index.md 规范（新建服务必遵；对齐 `anti-ddos/index.md`）

> 权威对照：当前 **C 仓**内已有分类页（如 `docs/{zh-cn|en-us}/best-practices/anti-ddos/index.md`）与 C 仓 / 本仓库 `templates/category_index*.md`。  
> **禁止**生成曾出现的精简版 index（见文末「错误示例」）。

仅 **服务目录首次出现** 时 `create` 全文；已有服务只 **update**「最佳实践列表 / Best Practices List」中的一条（含链接 + 一句话说明）。

### 英文（步骤 3）固定骨架与内容量

```markdown
# Introduction

## What is {ServiceName}

{两到三段产品介绍，勿压缩成一句}

## Best Practices Overview

{两段固定套话，见下}

## Best Practices List

This section contains the following best practices:

* [{Practice title = H1, e.g. Deploy Black/White Lists}]({practice}.md) - {one-sentence description covering main resources/steps}.

## Reference Materials

- [Huawei Cloud {ServiceName} Product Documentation](https://support.huaweicloud.com/{product}/index.html)
- [Terraform Official Documentation](https://www.terraform.io/docs/index.html)
```

| 章节 | 硬性要求 |
|------|----------|
| `# Introduction` | 固定；不要改成服务名 |
| `## What is …` | 标题取产品英文名；全称与缩写相同时可写 `## What is Anti-DDoS`，否则 `## What is Advanced Anti-DDoS (AAD)`。正文 **至少两段**（宜 2–3 段）：职责/能力 → 典型能力或防护/部署模式 → 可选运维价值。须像产品文档介绍，**不是**「用 Terraform 部署本实践」的一句话 |
| `## Best Practices Overview` | **必须两段**，套用下列句式（仅替换服务名与资源表述，勿自行改写成一句短述）： |
| | ① `This section provides best practice examples for using Terraform to automatically deploy and manage Huawei Cloud {Name}, helping you understand how to efficiently manage cloud {Name} … resources using Infrastructure as Code (IaC).` |
| | ② `Through the best practices in this section, you can learn the main deployment processes for {Name} … resources. These best practices will help you quickly get started with automated {Name} deployment and lay a solid foundation for subsequent {Name} management and operation work.` |
| `## Best Practices List` | 固定导语一行：`This section contains the following best practices:`。列表用 `*`。每条：`* [{与英文正文 H1 完全一致的标题}](file.md) - Introduces how to use Terraform to …`。**标题必须是 `Deploy …`，禁止 `AAD Black/White Lists` 这类缺 Deploy 的写法** |
| `## Reference Materials` | 两条：① 华为云该产品 Supports index；② **固定** `[Terraform Official Documentation](https://www.terraform.io/docs/index.html)`。**不要**在 index 放 Provider 文档链接（Provider 属于实践正文 Reference Information） |

### 中文（步骤 6）固定骨架与内容量

```markdown
# 简介

## 什么是{中文全称}（{简称}）

{两到三段产品介绍，勿压缩成一句}

## 最佳实践简述

{两段固定套话，见下}

## 最佳实践列表

本章节包含以下最佳实践：

* [{实践中文标题 = H1，如 部署黑白名单}]({practice}.md) - {一句话说明主要资源/步骤}。

## 参考资料

- [华为云{产品}产品文档](https://support.huaweicloud.com/{product}/index.html)
- [Terraform官方文档](https://www.terraform.io/docs/index.html)
```

| 章节 | 硬性要求 |
|------|----------|
| `# 简介` | 固定两字标题 |
| `## 什么是…` | **至少两段**产品介绍（对齐中文 Anti-DDoS），可引用华为云 Supports 语义，勿编造不存在的计费/规格细节 |
| `## 最佳实践简述` | **必须两段**：① `本章节提供了使用Terraform自动化部署和管理华为云{全称}（{简称}）的最佳实践示例，帮助您了解如何利用Infrastructure as Code（IaC）的方式高效地管理云上的{简称}…资源。` ② `通过本章节的最佳实践，您可以学习到主要的{简称}…资源的部署流程，这些最佳实践将帮助您快速上手{简称}的自动化部署，并为后续的…管理和运维工作奠定坚实基础。` |
| `## 最佳实践列表` | 固定导语：`本章节包含以下最佳实践：`。每条 `* [{与中文正文 H1 完全一致的标题}](file.md) - 介绍如何使用Terraform…`。**标题必须是 `部署…`，禁止 `AAD黑白名单最佳实践`（带「最佳实践」后缀或缺少「部署」）**；顺序与英文 index 一致 |
| `## 参考资料` | 产品文档 + **固定** Terraform 官方文档；不要用 Provider 文档替代第二条 |

### 错误示例（AAD 精简版 — 禁止再现）

```markdown
## What is Advanced Anti-DDoS (AAD)
Advanced Anti-DDoS (AAD) is a professional DDoS protection service…（仅一段）

## Best Practices Overview
This section provides best practices for deploying and configuring AAD instances using Terraform, including…（仅一句，未用固定套话）

## Best Practices List
- [Deploy AAD Black and White Lists](black_white_lists.md)   ← 缺导语、缺「 - 说明」、列表符号不规范

## Reference Materials
- … Product Documentation
- [Huawei Cloud Provider Documentation](…)   ← 错误：index 应用 Terraform Official Documentation
```

中英 `index.md` 须语义对齐、篇幅相当；不得英文写满、中文精简（或反之）。

---

## 硬性约束（输出）

1. **只输出一个 JSON 对象**，不要 Markdown 围栏或前后解释。
2. `files[]` **至少**包含步骤 1、2 的正文 `create`；新服务时还须包含中英 `index.md` 的 `create`。
3. 导航类 `update` 若输出，必须是基线 + 最小插入；**禁止**缩成短目录。
4. `path` 相对 C 仓根，禁止 `..`，且落在 `docs/zh-cn/` 或 `docs/en-us/`。
5. 参数与依赖必须来自源 HCL。
6. `summary` 用 **英文** 一句话（供 PR 说明）。

## 输出 schema（示意）

```json
{
  "practice_id": "examples/ecs/basic",
  "summary": "Add bilingual ECS basic instance best-practice docs and navigation",
  "files": [
    {
      "path": "docs/zh-cn/best-practices/ecs/simple_instance.md",
      "action": "create",
      "content": "……中文正文……\n"
    },
    {
      "path": "docs/en-us/best-practices/ecs/simple_instance.md",
      "action": "create",
      "content": "……English body……\n"
    }
  ]
}
```

新服务时继续附带中英 `index.md`（create）。`SUMMARY.md` / `README.md` 优先由编排层按步骤 3–8 补丁；模型输出导航时须遵守「英文定序、中文跟随」。
