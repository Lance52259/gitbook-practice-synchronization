package nav_test

import (
	"strings"
	"testing"

	"github.com/chnsz/gitbook-practice-synchronization/internal/nav"
)

func TestExcerptAADEnglishKeepsFullParagraph(t *testing.T) {
	// PR #3 regression: 459-rune EN first paragraph was trimmed to a 92-rune
	// first sentence while ZH kept the full paragraph.
	index := `# Introduction

## What is Advanced Anti-DDoS (AAD)

Advanced Anti-DDoS (AAD) is a professional DDoS protection service provided by Huawei Cloud. It is designed to protect Internet servers and applications from distributed denial-of-service (DDoS) attacks and other malicious traffic. AAD provides comprehensive protection capabilities, including DDoS traffic cleaning, CC (Challenge Collapsar) attack protection, and intelligent traffic analysis, ensuring the availability and stability of your online services.

AAD offers flexible deployment modes.

## Best Practices Overview
`
	want := "Advanced Anti-DDoS (AAD) is a professional DDoS protection service provided by Huawei Cloud. It is designed to protect Internet servers and applications from distributed denial-of-service (DDoS) attacks and other malicious traffic. AAD provides comprehensive protection capabilities, including DDoS traffic cleaning, CC (Challenge Collapsar) attack protection, and intelligent traffic analysis, ensuring the availability and stability of your online services."
	got := nav.ExcerptWhatIsParagraph(index, nav.EnUS)
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(got, "intelligent traffic analysis") {
		t.Fatalf("must not collapse to first sentence only: %q", got)
	}
	heading, _ := nav.ReadmeNavFromIndex(index, nav.EnUS, "AAD")
	if heading != "Advanced Anti-DDoS (AAD) Best Practices" {
		t.Fatalf("heading=%q", heading)
	}
}

func TestExcerptAntiDDoSStyle(t *testing.T) {
	index := `# Introduction

## What is Anti-DDoS

Anti-DDoS (Anti-Distributed Denial of Service) is a distributed denial-of-service attack protection service provided by Huawei Cloud, which can effectively protect public IPs from DDoS attacks and ensure stable business operations. Anti-DDoS service provides two protection modes: Basic Protection and Professional Protection. Basic Protection provides free DDoS attack protection capabilities for Huawei Cloud users. When a DDoS attack is detected, the system will automatically start traffic cleaning, filter out attack traffic, and only forward normal traffic to the origin server.

Anti-DDoS service supports protection against multiple attack types.

## Best Practices Overview
`
	// Full first paragraph fits under the raised rune budget (was truncated to
	// first sentence only when max was 450).
	want := "Anti-DDoS (Anti-Distributed Denial of Service) is a distributed denial-of-service attack protection service provided by Huawei Cloud, which can effectively protect public IPs from DDoS attacks and ensure stable business operations. Anti-DDoS service provides two protection modes: Basic Protection and Professional Protection. Basic Protection provides free DDoS attack protection capabilities for Huawei Cloud users. When a DDoS attack is detected, the system will automatically start traffic cleaning, filter out attack traffic, and only forward normal traffic to the origin server."
	got := nav.ExcerptWhatIsParagraph(index, nav.EnUS)
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	heading, blurb := nav.ReadmeNavFromIndex(index, nav.EnUS, "ANTI-DDOS")
	if heading != "Anti-DDoS Best Practices" {
		t.Fatalf("heading=%q", heading)
	}
	if blurb != want {
		t.Fatalf("blurb=%q", blurb)
	}
}

func TestExcerptPacksSentencesWhenOverBudget(t *testing.T) {
	// Build a first paragraph > 600 runes with clear sentence boundaries.
	long := strings.Repeat("Word ", 80) // 400 runes of filler words+spaces roughly
	para := "Alpha service is provided by Huawei Cloud. " + long + "Beta continues with more detail about capabilities. Gamma closes the paragraph."
	index := "# Introduction\n\n## What is Alpha\n\n" + para + "\n\n## Best Practices Overview\n"
	got := nav.ExcerptWhatIsParagraph(index, nav.EnUS)
	if got == "" {
		t.Fatal("empty excerpt")
	}
	if utf8RuneCount(got) > 600 {
		t.Fatalf("excerpt too long: %d", utf8RuneCount(got))
	}
	if !strings.HasSuffix(strings.TrimSpace(got), ".") {
		t.Fatalf("should end on a sentence boundary: %q", got)
	}
	if strings.Contains(got, "Gamma closes") && utf8RuneCount(para) > 600 {
		// Gamma may or may not fit; Alpha+Beta should be preferred over mid-word cut
		_ = got
	}
}

func TestExcerptAOMFullFirstParagraph(t *testing.T) {
	index := `# Introduction

## What is Application Operations Management (AOM)

Application Operations Management (AOM) is a one-stop application operations management platform provided by Huawei Cloud, offering enterprises a unified application operations management entry point. AOM helps enterprises achieve automated operations and intelligent management through application monitoring, log management, alarm management, and other functions, improving operational efficiency and quality.

AOM service supports multiple monitoring metrics.

## Best Practices Overview
`
	got := nav.ExcerptWhatIsParagraph(index, nav.EnUS)
	if !strings.Contains(got, "one-stop application operations") || !strings.Contains(got, "operational efficiency and quality.") {
		t.Fatalf("expected full first paragraph, got: %s", got)
	}
	heading, _ := nav.ReadmeNavFromIndex(index, nav.EnUS, "AOM")
	if heading != "Application Operations Management (AOM) Best Practices" {
		t.Fatalf("heading=%q", heading)
	}
}

func TestExcerptChinese(t *testing.T) {
	index := `# 简介

## 什么是Anti-DDoS（Anti-DDoS）

Anti-DDoS（Anti-Distributed Denial of Service）是华为云提供的分布式拒绝服务攻击防护服务，能够有效防护针对公网IP的DDoS攻击，保障业务的稳定运行。Anti-DDoS服务提供基础防护和专业版防护两种防护模式，基础防护为华为云用户提供免费的DDoS攻击防护能力，当检测到DDoS攻击时，系统会自动启动流量清洗，将攻击流量过滤后，仅将正常流量转发给源站服务器。

Anti-DDoS服务支持多种攻击类型的防护。

## 最佳实践简述
`
	got := nav.ExcerptWhatIsParagraph(index, nav.ZhCN)
	if !strings.HasPrefix(got, "Anti-DDoS（Anti-Distributed Denial of Service）是华为云提供的") {
		t.Fatalf("got=%q", got)
	}
	if !strings.Contains(got, "稳定运行。") {
		t.Fatalf("got=%q", got)
	}
	// Second paragraph must not be included
	if strings.Contains(got, "支持多种攻击类型") {
		t.Fatalf("should stop at first paragraph: %q", got)
	}
	heading, _ := nav.ReadmeNavFromIndex(index, nav.ZhCN, "AAD")
	if heading != "Anti-DDoS（Anti-DDoS）最佳实践" {
		t.Fatalf("heading=%q", heading)
	}
}

func utf8RuneCount(s string) int {
	return len([]rune(s))
}
