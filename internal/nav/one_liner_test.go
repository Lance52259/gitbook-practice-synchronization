package nav_test

import (
	"strings"
	"testing"

	"github.com/chnsz/gitbook-practice-synchronization/internal/nav"
)

func TestDefaultEnglishOneLiner(t *testing.T) {
	got := nav.DefaultEnglishOneLiner("Deploy Redis Big Key Analysis", nil)
	want := "Introduces how to use Terraform to automatically deploy Redis Big Key Analysis."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	got = nav.DefaultEnglishOneLiner("Deploy Master-Standby Redis Instance", []string{
		"VPC creation", "instance configuration", "backup policy", "whitelist management",
	})
	if !strings.HasPrefix(got, "Introduces how to use Terraform to automatically deploy Master-Standby Redis Instance, including ") {
		t.Fatalf("prefix: %q", got)
	}
	if !strings.Contains(got, "VPC creation") || !strings.Contains(got, "and whitelist management") {
		t.Fatalf("including phrase: %q", got)
	}
	if strings.ContainsAny(got, "«»「」") {
		t.Fatalf("must not quote title: %q", got)
	}
}

func TestDefaultChineseOneLiner(t *testing.T) {
	got := nav.DefaultChineseOneLiner("部署Redis大Key分析", []string{"大Key分析资源"})
	if got != "介绍如何使用Terraform自动化部署Redis大Key分析，包括大Key分析资源。" {
		t.Fatalf("got %q", got)
	}
}

func TestIsWeakOneLiner(t *testing.T) {
	if !nav.IsWeakOneLiner(`Introduces how to use Terraform to automate «Deploy Redis Backup».`) {
		t.Fatal("quoted automate form should be weak")
	}
	if !nav.IsWeakOneLiner(`Introduces how to use Terraform to automate Deploy Redis Backup.`) {
		t.Fatal("automate without including should be weak")
	}
	good := `Introduces how to use Terraform to automatically deploy DCS master-standby Redis instances, including VPC creation, instance configuration, backup policy, and whitelist management.`
	if nav.IsWeakOneLiner(good) {
		t.Fatal("valuable form must not be weak")
	}
}

func TestResourceHintsFromPractice(t *testing.T) {
	body := `# Deploy X

## Related Resources/Data Sources

### Resources

- [VPC (huaweicloud_vpc)](https://example.com)
- [DCS Instance (huaweicloud_dcs_instance)](https://example.com)
- [DCS Backup (huaweicloud_dcs_backup)](https://example.com)

## Operation Steps
`
	hints := nav.ResourceHintsFromPractice(body)
	joined := strings.Join(hints, "|")
	for _, want := range []string{"VPC creation", "instance configuration", "backup policy"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("hints=%v missing %q", hints, want)
		}
	}
}

func TestOneLinerFromIndexList(t *testing.T) {
	idx := `## Best Practices List

* [Deploy Redis Backup](redis_backup.md) - Introduces how to use Terraform to automatically deploy Redis backup, including backup policy.
`
	got := nav.OneLinerFromIndexList(idx, "redis_backup.md")
	if !strings.Contains(got, "including backup policy") {
		t.Fatalf("got %q", got)
	}
}
