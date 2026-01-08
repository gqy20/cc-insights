package main

import (
	"fmt"
	"io"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
)

// CreateCommandChart 创建命令使用统计图表
func CreateCommandChart(cmdStats []CommandStats) *charts.Bar {
	cmdNames := make([]string, 0, len(cmdStats))
	var cmdCounts []opts.BarData

	// 取前15个
	limit := 15
	if len(cmdStats) < limit {
		limit = len(cmdStats)
	}

	for i := 0; i < limit; i++ {
		cmdNames = append(cmdNames, cmdStats[i].Command)
		cmdCounts = append(cmdCounts, opts.BarData{Value: cmdStats[i].Count})
	}

	bar := charts.NewBar()

	bar.SetXAxis(cmdNames).
		AddSeries("使用次数", cmdCounts)

	bar.SetSeriesOptions(
		charts.WithLabelOpts(opts.Label{
			Show:     true,
			Position: "top",
		}),
	)

	// 全局选项
	bar.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title:    "Slash Commands 使用统计 (Top 15)",
			Subtitle: "数据来源: history.jsonl",
		}),
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  "wonderland",
			Width:  "1200px",
			Height: "500px",
		}),
	)

	return bar
}

// CreateDailyTrendChart 创建每日趋势图表
func CreateDailyTrendChart(dates []string, counts []int) *charts.Line {
	var lineData []opts.LineData
	for _, c := range counts {
		lineData = append(lineData, opts.LineData{Value: c})
	}

	line := charts.NewLine()
	line.SetXAxis(dates).AddSeries("消息数", lineData)

	line.SetSeriesOptions(
		charts.WithLabelOpts(opts.Label{
			Show: true,
		}),
		charts.WithAreaStyleOpts(opts.AreaStyle{
			Opacity: 0.2,
		}),
	)

	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title:    "每日活动趋势",
			Subtitle: "数据来源: projects/*.jsonl",
		}),
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  "wonderland",
			Width:  "1200px",
			Height: "400px",
		}),
	)

	return line
}

// CreateHourlyChart 创建小时分布图表
func CreateHourlyChart(hourlyCounts map[string]int) *charts.Bar {
	hours := make([]string, 24)
	var barData []opts.BarData

	for i := 0; i < 24; i++ {
		hours[i] = fmt.Sprintf("%02d:00", i)
		barData = append(barData, opts.BarData{Value: hourlyCounts[fmt.Sprintf("%02d", i)]})
	}

	bar := charts.NewBar()
	bar.SetXAxis(hours).AddSeries("活动次数", barData)

	bar.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title:    "24小时活动分布",
			Subtitle: "数据来源: history.jsonl",
		}),
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  "wonderland",
			Width:  "1200px",
			Height: "400px",
		}),
	)

	bar.SetSeriesOptions(
		charts.WithLabelOpts(opts.Label{
			Show: false,
		}),
	)

	return bar
}

// CreateMCPToolsChart 创建 MCP 工具使用图表
func CreateMCPToolsChart(toolStats []MCPToolStats) *charts.Pie {
	// 取前10个
	limit := 10
	if len(toolStats) < limit {
		limit = len(toolStats)
	}

	data := make([]opts.PieData, 0, limit)
	for i := 0; i < limit; i++ {
		label := toolStats[i].Server + "::" + toolStats[i].Tool
		data = append(data, opts.PieData{
			Name:  label,
			Value: toolStats[i].Count,
		})
	}

	pie := charts.NewPie()

	pie.AddSeries("MCP 工具调用", data).
		SetSeriesOptions(
			charts.WithLabelOpts(
				opts.Label{
					Show:      true,
					Formatter: "{b}: {c} ({d}%)",
				}),
		)

	pie.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title:    "MCP 工具调用统计 (Top 10)",
			Subtitle: "数据来源: debug/ 目录",
			Left:     "center",
			Top:      "20px",
		}),
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  "wonderland",
			Width:  "900px",
			Height: "700px",
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger:   "item",
			Formatter: "{b}: {c} ({d}%)",
		}),
	)

	return pie
}

// CreateDashboard 创建完整 Dashboard
func CreateDashboard() (*components.Page, error) {
	page := components.NewPage()
	page.SetLayout(components.PageCenterLayout)

	// 获取数据
	cmdStats, _, err := ParseHistory()
	if err != nil {
		return nil, err
	}

	dates, counts, err := GetDailyTrend()
	if err != nil {
		return nil, err
	}

	_, hourlyCounts, err := ParseHistory()
	if err != nil {
		return nil, err
	}

	toolStats, err := ParseDebugLogs()
	if err != nil {
		return nil, err
	}

	// 添加图表
	page.AddCharts(
		CreateDailyTrendChart(dates, counts),
		CreateCommandChart(cmdStats),
		CreateHourlyChart(hourlyCounts),
		CreateMCPToolsChart(toolStats),
	)

	return page, nil
}

// ServeDashboard 直接输出 Dashboard HTML
func ServeDashboard(output io.Writer) error {
	page, err := CreateDashboard()
	if err != nil {
		return err
	}

	page.Render(output)
	return nil
}
