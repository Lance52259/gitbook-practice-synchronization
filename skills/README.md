# Skills

本目录存放 gitbook-practice-synchronization 用于驱动 DeepSeek 生成文档的 Skill。

| Skill ID | 说明 |
|----------|------|
| `best-practice-doc` | 默认：对接 C 仓（`$C_REPO`，如 [chnsz/hcbp-demo](https://github.com/chnsz/hcbp-demo)）中文最佳实践文档体系 |

通过环境变量 `SKILL_ID` 或配置 `skill.id` 选择。

源脚本根目录由 `$B_REPO` + `$B_EXAMPLES_PATH`（及 `$B_DEFAULT_BRANCH`）决定（如 [huaweicloud/terraform-provider-huaweicloud](https://github.com/huaweicloud/terraform-provider-huaweicloud)），勿在 Skill 中写死具体仓库。
