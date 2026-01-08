package main

import (
	"time"
)

// RangePreset 时间范围预设
type RangePreset string

const (
	Range7Days   RangePreset = "7d"
	Range30Days  RangePreset = "30d"
	Range90Days  RangePreset = "90d"
	RangeAll     RangePreset = "all"
	RangeCustom  RangePreset = "custom"
)

// TimeFilter 时间过滤器
type TimeFilter struct {
	Start *time.Time
	End   *time.Time
}

// NewTimeFilterFromPreset 从预设创建时间过滤器
func NewTimeFilterFromPreset(preset RangePreset) TimeFilter {
	now := time.Now()
	var start time.Time

	switch preset {
	case Range7Days:
		start = now.AddDate(0, 0, -7)
		return TimeFilter{
			Start: &start,
			End:   &now,
		}
	case Range30Days:
		start = now.AddDate(0, 0, -30)
		return TimeFilter{
			Start: &start,
			End:   &now,
		}
	case Range90Days:
		start = now.AddDate(0, 0, -90)
		return TimeFilter{
			Start: &start,
			End:   &now,
		}
	case RangeAll:
		return TimeFilter{
			Start: nil,
			End:   nil,
		}
	default:
		return TimeFilter{
			Start: nil,
			End:   nil,
		}
	}
}

// NewTimeFilterCustom 创建自定义时间过滤器
func NewTimeFilterCustom(start, end string) (TimeFilter, error) {
	layout := "2006-01-02"
	s, err := time.Parse(layout, start)
	if err != nil {
		return TimeFilter{}, err
	}
	e, err := time.Parse(layout, end)
	if err != nil {
		return TimeFilter{}, err
	}
	// 设置结束时间为当天的 23:59:59
	e = time.Date(e.Year(), e.Month(), e.Day(), 23, 59, 59, 0, time.Local)

	return TimeFilter{
		Start: &s,
		End:   &e,
	}, nil
}

// Contains 检查时间是否在范围内
func (tf TimeFilter) Contains(t time.Time) bool {
	if tf.Start == nil && tf.End == nil {
		return true
	}

	if tf.Start != nil && t.Before(*tf.Start) {
		return false
	}

	if tf.End != nil && t.After(*tf.End) {
		return false
	}

	return true
}

// FilterHistoryRecords 过滤历史记录
func FilterHistoryRecords(records []HistoryRecord, tf TimeFilter) []HistoryRecord {
	if tf.Start == nil && tf.End == nil {
		return records
	}

	result := make([]HistoryRecord, 0)
	for _, record := range records {
		// timestamp 是毫秒
		t := time.Unix(record.Timestamp/1000, 0)
		if tf.Contains(t) {
			result = append(result, record)
		}
	}
	return result
}

// FilterDailyActivity 过滤每日活动
func FilterDailyActivity(activity []DailyActivity, tf TimeFilter) []DailyActivity {
	if tf.Start == nil && tf.End == nil {
		return activity
	}

	result := make([]DailyActivity, 0)
	for _, day := range activity {
		t, err := time.Parse("2006-01-02", day.Date)
		if err != nil {
			continue
		}
		if tf.Contains(t) {
			result = append(result, day)
		}
	}
	return result
}

// FilterDebugFiles 过滤 debug 文件列表（解析前过滤）
func FilterDebugFiles(fileInfos []DebugFileInfo, tf TimeFilter) []DebugFileInfo {
	if tf.Start == nil && tf.End == nil {
		return fileInfos
	}

	result := make([]DebugFileInfo, 0)
	for _, info := range fileInfos {
		if tf.Contains(info.ModTime) {
			result = append(result, info)
		}
	}
	return result
}
