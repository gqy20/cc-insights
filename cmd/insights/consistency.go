package main

import "fmt"

type DashboardConsistencyReport struct {
	Checks map[string]int `json:"checks"`
	Issues []string       `json:"issues"`
}

func validateDashboardDataConsistency(data *DashboardData) []string {
	return buildDashboardConsistencyReport(data).Issues
}

func buildDashboardConsistencyReport(data *DashboardData) DashboardConsistencyReport {
	report := DashboardConsistencyReport{
		Checks: make(map[string]int),
	}
	if data == nil {
		report.Issues = append(report.Issues, "dashboard data is nil")
		return report
	}

	totalMessages := 0
	if data.ProjectStats != nil {
		totalMessages = data.ProjectStats.TotalMessages
		for _, project := range data.ProjectStats.Projects {
			report.Checks["project_items_sum"] += project.MessageCount
		}
	}
	report.Checks["total_messages"] = totalMessages

	for _, count := range data.DailyTrend.Counts {
		report.Checks["daily_sum"] += count
	}
	for _, count := range data.HourlyCounts {
		report.Checks["hourly_sum"] += count
	}
	if data.WeekdayStats != nil {
		for _, weekday := range data.WeekdayStats.WeekdayData {
			report.Checks["weekday_sum"] += weekday.MessageCount
		}
	}
	for _, model := range data.ModelUsage {
		report.Checks["model_usage_sum"] += model.Count
	}
	if data.WorkHoursStats != nil {
		report.Checks["work_hours_sum"] = data.WorkHoursStats.WorkHoursCount + data.WorkHoursStats.OffHoursCount
		workHourlySum := 0
		for _, hour := range data.WorkHoursStats.HourlyData {
			workHourlySum += hour.Count
		}
		report.Checks["work_hourly_sum"] = workHourlySum
	}
	if data.CostAnalysis != nil {
		report.Checks["cost_request_count"] = data.CostAnalysis.Totals.RequestCount
		for _, model := range data.CostAnalysis.ByModel {
			report.Checks["cost_by_model_request_sum"] += model.RequestCount
		}
	}
	if data.ToolAnalysis != nil {
		report.Checks["tool_total_calls"] = data.ToolAnalysis.TotalCalls
		report.Checks["tool_total_failures"] = data.ToolAnalysis.TotalFailures
		report.Checks["tool_missing_results"] = data.ToolAnalysis.MissingResults
		for _, tool := range data.ToolAnalysis.Tools {
			report.Checks["tool_items_call_sum"] += tool.CallCount
			report.Checks["tool_items_failure_sum"] += tool.FailureCount
			report.Checks["tool_items_missing_sum"] += tool.MissingResultCount
		}
	}
	for _, tool := range data.MCPTools {
		report.Checks["mcp_tool_sum"] += tool.Count
	}
	if data.ToolPerformance != nil {
		report.Checks["tool_perf_paired_calls"] = data.ToolPerformance.TotalPairedCalls
		report.Checks["tool_perf_errors"] = data.ToolPerformance.TotalErrors
		for _, category := range data.ToolPerformance.ByCategory {
			report.Checks["tool_perf_category_call_sum"] += category.CallCount
			report.Checks["tool_perf_category_error_sum"] += category.ErrorCount
			report.Checks["tool_perf_category_missing_sum"] += category.MissingCount
		}
	}

	report.expectEqual("daily_sum", "total_messages")
	report.expectEqual("project_items_sum", "total_messages")
	report.expectEqual("model_usage_sum", "total_messages")
	report.expectEqual("hourly_sum", "total_messages")
	report.expectEqual("weekday_sum", "total_messages")
	report.expectEqual("work_hours_sum", "total_messages")
	report.expectEqual("work_hourly_sum", "total_messages")
	report.expectEqual("cost_request_count", "total_messages")
	report.expectEqual("cost_by_model_request_sum", "cost_request_count")
	report.expectEqual("tool_items_call_sum", "tool_total_calls")
	report.expectEqual("tool_items_failure_sum", "tool_total_failures")
	report.expectEqual("tool_items_missing_sum", "tool_missing_results")
	report.expectLessOrEqual("mcp_tool_sum", "tool_total_calls")
	report.expectLessOrEqual("tool_perf_paired_calls", "tool_total_calls")
	report.expectLessOrEqual("tool_perf_errors", "tool_perf_paired_calls")
	report.expectLessOrEqual("tool_perf_category_error_sum", "tool_perf_category_call_sum")
	report.expectLessOrEqual("tool_perf_category_missing_sum", "tool_perf_category_call_sum")

	return report
}

func (r *DashboardConsistencyReport) expectEqual(left, right string) {
	leftValue, leftOK := r.Checks[left]
	rightValue, rightOK := r.Checks[right]
	if !leftOK || !rightOK {
		return
	}
	if leftValue != rightValue {
		r.Issues = append(r.Issues, fmt.Sprintf("%s=%d != %s=%d", left, leftValue, right, rightValue))
	}
}

func (r *DashboardConsistencyReport) expectLessOrEqual(left, right string) {
	leftValue, leftOK := r.Checks[left]
	rightValue, rightOK := r.Checks[right]
	if !leftOK || !rightOK {
		return
	}
	if leftValue > rightValue {
		r.Issues = append(r.Issues, fmt.Sprintf("%s=%d > %s=%d", left, leftValue, right, rightValue))
	}
}
