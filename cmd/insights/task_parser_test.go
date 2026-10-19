package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRecordClaudeTasksIntoAgg claude tasks/ 扫描写入 TaskAgg（顶层 + 按天快照），
// 时间过滤查询经按天快照重建 TaskPlanAnalysis。
func TestRecordClaudeTasksIntoAgg(t *testing.T) {
	dataDir := t.TempDir()
	sessionDir := filepath.Join(dataDir, "tasks", "sess-t1")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mtime := time.Date(2026, 6, 15, 10, 0, 0, 0, time.Local)
	writeTask := func(name, status string) {
		path := filepath.Join(sessionDir, name)
		if err := os.WriteFile(path, []byte(`{"id":"`+name+`","subject":"`+name+`","status":"`+status+`"}`), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}
	writeTask("a.json", "completed")
	writeTask("b.json", "pending")

	agg := newProjectAggregate()
	if err := recordClaudeTasksIntoAgg(dataDir, agg); err != nil {
		t.Fatalf("recordClaudeTasksIntoAgg: %v", err)
	}
	agg.finalize()

	if agg.TaskAgg == nil || agg.TaskAgg.TotalTasks != 2 {
		t.Fatalf("TaskAgg = %#v, want 2 tasks", agg.TaskAgg)
	}
	if agg.TaskAgg.SessionTasks["sess-t1"] == nil || agg.TaskAgg.SessionTasks["sess-t1"].CompletedCount != 1 {
		t.Fatalf("session tasks = %#v, want 1 completed", agg.TaskAgg.SessionTasks)
	}
	daily := agg.DailyRuntime["2026-06-15"]
	if daily.TaskAgg == nil || daily.TaskAgg.TotalTasks != 2 {
		t.Fatalf("daily TaskAgg = %#v, want 2 tasks on 2026-06-15", daily.TaskAgg)
	}
	tasks := agg.TaskPlanAnalysis.Tasks
	if tasks.TotalTasks != 2 || tasks.TotalSessions != 1 {
		t.Fatalf("Tasks = %#v, want 2 tasks / 1 session", tasks)
	}
	if tasks.CompletionRate < 49.9 || tasks.CompletionRate > 50.1 {
		t.Fatalf("completion rate = %.1f, want 50", tasks.CompletionRate)
	}

	// 时间过滤重建：经序列化-还原 + merge 后 finalize 仍能给出任务统计。
	restored := projectFileAggregateToAggregate(aggregateToProjectFileAggregate(agg))
	dst := newProjectAggregate()
	mergeProjectAggregate(dst, restored)
	dst.finalize()
	if dst.TaskPlanAnalysis.Tasks.TotalTasks != 2 {
		t.Fatalf("rebuilt Tasks = %#v, want 2", dst.TaskPlanAnalysis.Tasks)
	}
}
