package model_test

import (
	"testing"

	"github.com/Lance52259/gitbook-practice-synchronization/internal/model"
)

func TestSimpleTitleAndCommitTitle(t *testing.T) {
	cases := []struct {
		id, wantTitle, wantCommit string
	}{
		{
			"examples/rds/basic_instance",
			"basic instance",
			"docs(rds): support new best practice for basic instance",
		},
		{
			"examples/rds/rds_mysql_single_instance",
			"mysql single instance",
			"docs(rds): support new best practice for mysql single instance",
		},
		{
			"examples/ecs/instance-with-eip",
			"instance with eip",
			"docs(ecs): support new best practice for instance with eip",
		},
		{
			"examples/ecs/ecs-simple-instance",
			"simple instance",
			"docs(ecs): support new best practice for simple instance",
		},
		{
			"examples/sfs-turbo/sfs_turbo_file_system",
			"file system",
			"docs(sfs-turbo): support new best practice for file system",
		},
		{
			"examples/sfs-turbo/file_system",
			"file system",
			"docs(sfs-turbo): support new best practice for file system",
		},
		{
			"examples/anti-ddos/basic",
			"basic",
			"docs(anti-ddos): support new best practice for basic",
		},
	}
	for _, tc := range cases {
		p := model.Practice{PracticeID: tc.id, SourcePath: tc.id}
		if got := p.SimpleTitle(); got != tc.wantTitle {
			t.Errorf("%s SimpleTitle=%q want %q", tc.id, got, tc.wantTitle)
		}
		if got := p.CommitTitle(); got != tc.wantCommit {
			t.Errorf("%s CommitTitle=%q want %q", tc.id, got, tc.wantCommit)
		}
	}
}
