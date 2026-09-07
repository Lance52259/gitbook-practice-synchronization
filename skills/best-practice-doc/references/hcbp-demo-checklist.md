# hcbp-demo 变更清单速查（中英双语）

来源约定见 C 仓（`$C_REPO`，如 [chnsz/hcbp-demo](https://github.com/chnsz/hcbp-demo)）的
[.cursor/rules/best-practice-style.mdc](https://github.com/chnsz/hcbp-demo/blob/master/.cursor/rules/best-practice-style.mdc) 与模板：

- [templates/best_practice.md](https://github.com/chnsz/hcbp-demo/blob/master/templates/best_practice.md)
- [templates/category_index.md](https://github.com/chnsz/hcbp-demo/blob/master/templates/category_index.md)

## 强制顺序（1 → 8）

英文侧（2–5）先按**英文字母序**定目录；中文侧（6–8）**跟随**英文顺序。

| # | 文件 | 动作 |
|---|------|------|
| 1 | `docs/zh-cn/best-practices/{service}/{practice}.md` | create |
| 2 | `docs/en-us/best-practices/{service}/{practice}.md` | create |
| 3 | `docs/en-us/best-practices/{service}/index.md` | create（新服务：**Anti-DDoS 同等篇幅**）或 update 列表项（含说明） |
| 4 | `docs/en-us/best-practices/README.md` | update 文档导航（**新服务必做**；简介从 index `## What is` 首段截取） |
| 5 | `docs/en-us/SUMMARY.md` | update：插入实践行；**新服务**再插入服务块 |
| 6 | `docs/zh-cn/best-practices/{service}/index.md` | create（新服务：同等规范）或 update（顺序对齐英文 index） |
| 7 | `docs/zh-cn/best-practices/README.md` | update（简介从 index `## 什么是` 首段截取；顺序对齐英文 README；新服务必做） |
| 8 | `docs/zh-cn/SUMMARY.md` | update（对齐英文 SUMMARY；新服务含服务块） |

## 红线

- 中英 `SUMMARY.md` 一定存在，只定点插入，禁止整文件重写
- 禁止只生成中文、省略英文
- 禁止先改中文导航再改英文导航
- 英文源码参考链接锚文本：`Best Practice Source Code Reference For {ServiceName} {PracticeObject}`（如 `… For AAD Black/White Lists`；**不要**带 `Deploy`；勿颠倒语序）
- 中文源码参考链接锚文本：`{服务名}{场景}最佳实践源码参考`（「最佳实践」只出现在源码锚文本，不出现在正文 H1）
- 正文 H1 / index 列表：中文 `部署…`、英文 `Deploy …`；禁止 `AAD黑白名单最佳实践` / `AAD Black/White Lists`
- 无 data source：导语用「主要资源」/ `main resources`，禁止空 `### 数据源`
- tfvars 步骤标题含「（可选）」/ `Required … (Optional)`
- 步骤 HCL 内联 `variable`；无 region 参数时用完整 region 缺省继承注释
- 同一 `variable "name"` 全文仅声明一次；后续步骤禁止重复粘贴（生成后自检去重）
- 中英 `best-practices/README.md` 新服务简介须从 index「什么是 / What is」首段截取，禁止 Terraform 占位句
- 新建 `index.md`：What is ≥2 段；Overview/简述必须两段固定套话；List 含导语 + `* [与 H1 一致的标题](file.md) - 说明`；参考资料第二条为 Terraform 官方文档（非 Provider）
- 禁止 AAD 式精简 index（一句 Overview、裸链接列表、Reference 放 Provider）

## 示例对照

- 中文正文：`docs/zh-cn/best-practices/ecs/simple_instance.md`
- 英文正文：`docs/en-us/best-practices/ecs/simple_instance.md`
- 分类页（权威）：`docs/{zh-cn|en-us}/best-practices/anti-ddos/index.md`
- 源脚本根：由 `$B_REPO` + `$B_DEFAULT_BRANCH` + `$B_EXAMPLES_PATH` 决定（如 https://github.com/huaweicloud/terraform-provider-huaweicloud/tree/master/examples）
