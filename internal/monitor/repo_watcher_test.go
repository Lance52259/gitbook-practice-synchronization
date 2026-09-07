package monitor

import "testing"

func TestWorktreeDirName(t *testing.T) {
	cases := []struct {
		role, repo, want string
	}{
		{"b", "huaweicloud/terraform-provider-huaweicloud", "b-huaweicloud-terraform-provider-huaweicloud"},
		{"c", "chnsz/hcbp-demo", "c-chnsz-hcbp-demo"},
		{"b", "https://github.com/org/repo.git", "b-org-repo"},
	}
	for _, tc := range cases {
		got := worktreeDirName(tc.role, tc.repo)
		if got != tc.want {
			t.Fatalf("role=%s repo=%s got %q want %q", tc.role, tc.repo, got, tc.want)
		}
	}
}
