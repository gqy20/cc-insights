package main

import (
	"testing"
	"time"
)

// 预设窗口曾被实现成 "End = 今天 00:00"，于是当天所有活动都被排除，
// 表现为 24h 的结果集反而大于 30d。窗口必须覆盖到当下，且彼此嵌套。
func TestPresetRangesIncludeToday(t *testing.T) {
	now := time.Now()
	presets := []RangePreset{Range7Days, Range30Days, Range90Days}

	var previousStart *time.Time
	for _, preset := range presets {
		tf := NewTimeFilterFromPreset(preset)
		if tf.Start == nil || tf.End == nil {
			t.Fatalf("%s 缺少边界: %+v", preset, tf)
		}
		if tf.End.Before(now) {
			t.Fatalf("%s 的 End=%v 早于现在，当天数据会被整段排除", preset, *tf.End)
		}
		if !tf.Contains(now) {
			t.Fatalf("%s 不含现在时刻", preset)
		}
		if tf.Start.After(now) {
			t.Fatalf("%s 的 Start=%v 晚于现在", preset, *tf.Start)
		}
		// 7d ⊂ 30d ⊂ 90d：起点只能越拉越早。
		if previousStart != nil && tf.Start.After(*previousStart) {
			t.Fatalf("%s 的 Start=%v 晚于上一个窗口的起点 %v，嵌套关系破了", preset, *tf.Start, *previousStart)
		}
		start := *tf.Start
		previousStart = &start
	}
}

func TestPresetAllLeavesRangeOpen(t *testing.T) {
	all := NewTimeFilterFromPreset(RangeAll)
	if all.Start != nil || all.End != nil {
		t.Fatalf("all 不应设边界: %+v", all)
	}
	if !all.Contains(time.Now()) || !all.Contains(time.Now().AddDate(-20, 0, 0)) {
		t.Fatalf("all 应包含任意时刻")
	}
}
