package nav_test

import (
	"strings"
	"testing"

	"github.com/chnsz/gitbook-practice-synchronization/internal/nav"
)

const sampleSUMMARY = `# Summary

* [产品介绍](introductions/)
  * [Terraform简介](introductions/terraform_introduction.md)
* [最佳实践](best-practices/)
  * [简介](best-practices/README.md)
  * [Anti-DDoS](best-practices/anti-ddos/)
    * [简介](best-practices/anti-ddos/index.md)
    * [部署基础防护](best-practices/anti-ddos/basic.md)
  * [AOM](best-practices/aom/)
    * [简介](best-practices/aom/index.md)
    * [部署AOM告警动作回调](best-practices/aom/action_callback.md)
* [贡献](contribute.md)
`

const sampleEnSUMMARY = `# Summary

* [Product Introduction](introductions/)
* [Best Practices](best-practices/)
  * [Introduction](best-practices/README.md)
  * [Anti-DDoS](best-practices/anti-ddos/)
    * [Introduction](best-practices/anti-ddos/index.md)
    * [Deploy Basic Protection](best-practices/anti-ddos/basic.md)
  * [AOM](best-practices/aom/)
    * [Introduction](best-practices/aom/index.md)
    * [Deploy AOM Alarm Action Callback](best-practices/aom/action_callback.md)
`

func TestPatchSUMMARYExistingService(t *testing.T) {
	got, err := nav.PatchSUMMARY(sampleSUMMARY, "aom", "AOM", "prevent_elb_alarm_storm", "部署AOM防止ELB告警风暴", nav.ZhCN.IntroLabel)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "best-practices/aom/prevent_elb_alarm_storm.md") {
		t.Fatalf("missing practice:\n%s", got)
	}
	if !strings.Contains(got, "best-practices/anti-ddos/basic.md") || !strings.Contains(got, "* [贡献]") {
		t.Fatalf("destroyed existing entries:\n%s", got)
	}
}

func TestPatchSUMMARYNewService(t *testing.T) {
	got, err := nav.PatchSUMMARY(sampleEnSUMMARY, "aad", "AAD", "black_white_lists", "Deploy Black/White Lists", nav.EnUS.IntroLabel)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "  * [AAD](best-practices/aad/)") || !strings.Contains(got, "[Introduction](best-practices/aad/index.md)") {
		t.Fatalf("missing service:\n%s", got)
	}
	aad := strings.Index(got, "best-practices/aad/")
	anti := strings.Index(got, "best-practices/anti-ddos/")
	if aad < 0 || anti < 0 || aad > anti {
		t.Fatalf("service order wrong:\n%s", got)
	}
}

func TestPatchServiceIndex(t *testing.T) {
	base := `# Introduction

## Best Practices List

This section contains the following best practices:

* [Deploy Basic Protection](basic.md) - Basic.
* [Deploy LTS Configuration](lts_config.md) - LTS.

## Reference Materials
`
	got, err := nav.PatchServiceIndex(base, "default_protection_policy", "Deploy Default Protection Policy", "Default policy", nav.EnUS)
	if err != nil {
		t.Fatal(err)
	}
	basic := strings.Index(got, "(basic.md)")
	def := strings.Index(got, "(default_protection_policy.md)")
	lts := strings.Index(got, "(lts_config.md)")
	if basic < 0 || def < 0 || lts < 0 || !(basic < def && def < lts) {
		t.Fatalf("order:\n%s", got)
	}
}

// Filename order would put redis_all_sessions_kill before redis_background_*; title order must be Account → Background → Instance.
func TestPatchSUMMARYByEnglishTitleNotFilename(t *testing.T) {
	base := `# Summary

* [Best Practices](best-practices/)
  * [Introduction](best-practices/README.md)
  * [DCS](best-practices/dcs/)
    * [Introduction](best-practices/dcs/index.md)
    * [Deploy Redis Account](best-practices/dcs/redis_account.md)
    * [Deploy Redis Instance All Sessions Kill](best-practices/dcs/redis_all_sessions_kill.md)
`
	got, err := nav.PatchSUMMARY(base, "dcs", "DCS", "redis_background_task_delete", "Deploy Redis Background Task Delete", nav.EnUS.IntroLabel)
	if err != nil {
		t.Fatal(err)
	}
	account := strings.Index(got, "redis_account.md")
	bg := strings.Index(got, "redis_background_task_delete.md")
	inst := strings.Index(got, "redis_all_sessions_kill.md")
	if account < 0 || bg < 0 || inst < 0 || !(account < bg && bg < inst) {
		t.Fatalf("want Account < Background < Instance by English title:\n%s", got)
	}
}

func TestPatchSUMMARYFollowOrderForZH(t *testing.T) {
	enOrder := []string{"redis_account.md", "redis_background_task_delete.md", "redis_all_sessions_kill.md"}
	zhBase := `# Summary

* [最佳实践](best-practices/)
  * [简介](best-practices/README.md)
  * [DCS](best-practices/dcs/)
    * [简介](best-practices/dcs/index.md)
    * [部署 Redis 账号](best-practices/dcs/redis_account.md)
    * [部署 Redis 实例杀掉所有会话](best-practices/dcs/redis_all_sessions_kill.md)
`
	got, err := nav.PatchSUMMARYFollowOrder(zhBase, "dcs", "DCS", "redis_background_task_delete", "部署 Redis 后台任务删除", nav.ZhCN.IntroLabel, enOrder)
	if err != nil {
		t.Fatal(err)
	}
	account := strings.Index(got, "redis_account.md")
	bg := strings.Index(got, "redis_background_task_delete.md")
	inst := strings.Index(got, "redis_all_sessions_kill.md")
	if account < 0 || bg < 0 || inst < 0 || !(account < bg && bg < inst) {
		t.Fatalf("ZH must follow EN file order, not Chinese title sort:\n%s", got)
	}
}

func TestPatchServiceIndexByEnglishTitle(t *testing.T) {
	base := `# Introduction

## Best Practices List

This section contains the following best practices:

* [Deploy Redis Account](redis_account.md) - Account.
* [Deploy Redis Instance All Sessions Kill](redis_all_sessions_kill.md) - Kill sessions.

`
	got, err := nav.PatchServiceIndex(base, "redis_background_task_delete", "Deploy Redis Background Task Delete", "Background", nav.EnUS)
	if err != nil {
		t.Fatal(err)
	}
	account := strings.Index(got, "(redis_account.md)")
	bg := strings.Index(got, "(redis_background_task_delete.md)")
	inst := strings.Index(got, "(redis_all_sessions_kill.md)")
	if account < 0 || bg < 0 || inst < 0 || !(account < bg && bg < inst) {
		t.Fatalf("want Account < Background < Instance:\n%s", got)
	}
}

func TestPatchBestPracticesREADME(t *testing.T) {
	base := `# Center

## Documentation Navigation

### [Anti-DDoS Best Practices](anti-ddos/index.md)

Anti-DDoS.

### [AOM Best Practices](aom/index.md)

AOM.

## Other
`
	got, err := nav.PatchBestPracticesREADME(base, "aad", "AAD Best Practices", "AAD intro.", nav.EnUS)
	if err != nil {
		t.Fatal(err)
	}
	aad := strings.Index(got, "(aad/index.md)")
	anti := strings.Index(got, "(anti-ddos/index.md)")
	if aad < 0 || anti < 0 || aad > anti {
		t.Fatalf("nav order:\n%s", got)
	}
	if !strings.Contains(got, "## Other") {
		t.Fatalf("lost suffix:\n%s", got)
	}
}

func TestPatchBestPracticesREADMERefreshesExisting(t *testing.T) {
	base := `# Center

## Documentation Navigation

### [Advanced Anti-DDoS (AAD) Best Practices](aad/index.md)

Advanced Anti-DDoS (AAD) is a professional DDoS protection service provided by Huawei Cloud.

### [Anti-DDoS Best Practices](anti-ddos/index.md)

Anti-DDoS.
`
	full := "Advanced Anti-DDoS (AAD) is a professional DDoS protection service provided by Huawei Cloud. It is designed to protect Internet servers."
	got, err := nav.PatchBestPracticesREADME(base, "aad", "Advanced Anti-DDoS (AAD) Best Practices", full, nav.EnUS)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "It is designed to protect") {
		t.Fatalf("should refresh blurb:\n%s", got)
	}
	if strings.Count(got, "(aad/index.md)") != 1 {
		t.Fatalf("duplicate aad block:\n%s", got)
	}
}

func TestIsDestructiveUpdate(t *testing.T) {
	if !nav.IsDestructiveUpdate(sampleSUMMARY, "# 目录\n\n* [AAD](best-practices/aad/)\n") {
		t.Fatal("expected destructive")
	}
	patched, _ := nav.PatchSUMMARY(sampleSUMMARY, "aad", "AAD", "black_white_lists", "部署黑白名单防护", nav.ZhCN.IntroLabel)
	if nav.IsDestructiveUpdate(sampleSUMMARY, patched) {
		t.Fatal("surgical patch should not be destructive")
	}
}
