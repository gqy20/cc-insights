package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseProjectsConcurrentOnce_SkillAnalysis(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	projectDir := filepath.Join(dataDir, "projects", "skill-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("创建项目目录失败: %v", err)
	}
	writeSkillFixture(t, dataDir, "playwright-cli")
	writeSkillFixture(t, dataDir, "tdd")

	base := time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC)
	content := attachmentRecord("/tmp/skill-project", "session-skill", base, `{"type":"skill_listing","names":["playwright-cli","write"],"skillCount":2,"isInitial":true}`) + "\n" +
		attachmentRecord("/tmp/skill-project", "session-skill", base.Add(time.Second), `{"type":"invoked_skills","skills":[{"name":"write","path":"userSettings:write"}]}`) + "\n" +
		toolUseRecordWithInput("/tmp/skill-project", "session-skill", base.Add(2*time.Second), "skill-ok", "Skill", "claude-sonnet-4.5", false, "", `{"skill":"playwright-cli","args":"open page and screenshot"}`) + "\n" +
		toolResultRecord("/tmp/skill-project", "session-skill", base.Add(3*time.Second), "skill-ok", "skill completed") + "\n" +
		toolUseRecordWithInput("/tmp/skill-project", "session-skill", base.Add(4*time.Second), "bash-fail", "Bash", "claude-sonnet-4.5", false, "", `{"command":"exit 1"}`) + "\n" +
		toolResultRecord("/tmp/skill-project", "session-skill", base.Add(5*time.Second), "bash-fail", "Error: exit code 1") + "\n" +
		toolUseRecordWithInput("/tmp/skill-project", "session-skill", base.Add(6*time.Second), "skill-missing", "Skill", "claude-sonnet-4.5", false, "", `{"skill":"tdd"}`) + "\n"

	if err := os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("写入测试 jsonl 失败: %v", err)
	}

	agg, err := ParseProjectsConcurrentOnceFromDir(TimeFilter{}, dataDir)
	if err != nil {
		t.Fatalf("ParseProjectsConcurrentOnceFromDir failed: %v", err)
	}
	if agg.SkillAnalysis == nil {
		t.Fatal("SkillAnalysis should not be nil")
	}
	if agg.SkillAnalysis.TotalInstalled != 2 {
		t.Fatalf("TotalInstalled=%d, want 2", agg.SkillAnalysis.TotalInstalled)
	}
	if agg.SkillAnalysis.ListingEvents != 1 || agg.SkillAnalysis.InitialListingEvents != 1 {
		t.Fatalf("listing events=(%d,%d), want (1,1)", agg.SkillAnalysis.ListingEvents, agg.SkillAnalysis.InitialListingEvents)
	}

	skills := map[string]SkillUsageStat{}
	for _, item := range agg.SkillAnalysis.Skills {
		skills[item.Name] = item
	}
	if skills["playwright-cli"].ToolUseCount != 1 || skills["playwright-cli"].SuccessCount != 1 {
		t.Fatalf("playwright-cli stats=%+v, want one successful Skill tool_use", skills["playwright-cli"])
	}
	if skills["write"].AttachmentCount != 1 {
		t.Fatalf("write stats=%+v, want one invoked_skills attachment", skills["write"])
	}
	if skills["tdd"].MissingResultCount != 1 {
		t.Fatalf("tdd stats=%+v, want one missing Skill result", skills["tdd"])
	}

	listing := map[string]int{}
	for _, item := range agg.SkillAnalysis.ListingSkills {
		listing[item.Name] = item.Count
	}
	if listing["playwright-cli"] != 1 || listing["write"] != 1 {
		t.Fatalf("ListingSkills=%+v, want playwright-cli/write once", agg.SkillAnalysis.ListingSkills)
	}

	chains := map[string]SkillToolChainStat{}
	for _, item := range agg.SkillAnalysis.ToolChains {
		chains[item.SkillName+"::"+item.Tool] = item
	}
	if chains["playwright-cli::Bash"].CallCount != 1 || chains["playwright-cli::Bash"].FailureCount != 1 {
		t.Fatalf("playwright-cli Bash chain=%+v, want one failed Bash", chains["playwright-cli::Bash"])
	}
}

func writeSkillFixture(t *testing.T, dataDir, name string) {
	t.Helper()
	dir := filepath.Join(dataDir, "skills", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("创建 skill 目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name+"\n"), 0644); err != nil {
		t.Fatalf("写入 SKILL.md 失败: %v", err)
	}
}
