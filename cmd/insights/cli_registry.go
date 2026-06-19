package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

// Command 描述一个 CLI 子命令的声明与执行。
// Name/Short/Long/Examples 构成帮助文本的单一事实源；
// Flags 注册该命令支持的 flag（经 fs.VisitAll 即为该命令的参数列表）；
// Run 自包含数据准备、构建与输出。
type Command struct {
	Name     string
	Short    string
	Long     string
	Examples []string
	Flags    func(*flag.FlagSet, *cliOptions)
	Run      func(opts cliOptions) error
}

// errHelp 表示帮助文本已打印，runCLI 应静默退出（退出码 0）。
var errHelp = errors.New("help shown")

var commands = []*Command{cmdSum, cmdErr, cmdWhy, cmdTok, cmdCmd, cmdSes, cmdRec, cmdWeb}

func lookupCommand(name string) *Command {
	for _, c := range commands {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// registerCommonAnalysisFlags 注册所有分析命令共用的 flag（时间范围与输出）。
// 注意：-j/-m 由 parseCLIOptions 注册（需保留指针做格式后处理），故不在此处。
func registerCommonAnalysisFlags(fs *flag.FlagSet, opts *cliOptions) {
	fs.StringVar(&opts.Preset, "preset", opts.Preset, "时间范围: 24h|7d|30d|90d|all")
	fs.StringVar(&opts.Preset, "p", opts.Preset, "短参数: --preset")
	fs.StringVar(&opts.Start, "start", "", "自定义开始日期 YYYY-MM-DD")
	fs.StringVar(&opts.End, "end", "", "自定义结束日期 YYYY-MM-DD")
	fs.StringVar(&opts.Format, "format", opts.Format, "输出格式: table|json|markdown")
	fs.StringVar(&opts.Format, "f", opts.Format, "短参数: --format")
	fs.IntVar(&opts.Limit, "limit", opts.Limit, "Top N 结果数量")
	fs.IntVar(&opts.Limit, "n", opts.Limit, "短参数: --limit")
}

// fastRun 是 err/why/tok/cmd/ses 共用的执行模板：
// 复用缓存快照的数据准备（轻量路径）+ 命令特定的构建闭包。
func fastRun(build func(*DashboardData, cliOptions) error) func(cliOptions) error {
	return func(opts cliOptions) error {
		tf, preset, err := timeFilterFromCLIOptions(opts)
		if err != nil {
			return err
		}
		if err := prepareCLIDataForRecommendations(); err != nil {
			return err
		}
		defer CloseLogger()
		data, _, err := buildRecommendationDashboardData(tf, preset)
		if err != nil {
			return err
		}
		return build(data, opts)
	}
}

var cmdSum = &Command{
	Name:     "sum",
	Short:    "全局使用概览",
	Long:     "汇总时间范围内的消息数、会话数、命令数、工具调用、Token 消耗、失败率以及主要项目/模型，作为整体用法的入口快照。",
	Examples: []string{"cc-insights", "cc-insights sum -p 30d -j"},
	Flags:    registerCommonAnalysisFlags,
	Run: func(opts cliOptions) error {
		tf, preset, err := timeFilterFromCLIOptions(opts)
		if err != nil {
			return err
		}
		if err := prepareCLIData(); err != nil {
			return err
		}
		defer CloseLogger()
		data, _, err := buildDashboardData(tf, preset)
		if err != nil {
			return err
		}
		return outputCLI(buildCLISummary(data), opts.Format, os.Stdout)
	},
}

var cmdErr = &Command{
	Name:     "err",
	Short:    "失败原因聚合统计",
	Long:     "按 reason、工具×原因、模型×工具等维度聚合失败，给出失败率与排行。回答「失败了多少、按什么分布、哪些组合最容易失败」。",
	Examples: []string{"cc-insights err -p 7d -j"},
	Flags:    registerCommonAnalysisFlags,
	Run: fastRun(func(data *DashboardData, opts cliOptions) error {
		return outputCLI(buildCLIFailureReport(data, opts.Limit), opts.Format, os.Stdout)
	}),
}

var cmdWhy = &Command{
	Name:  "why",
	Short: "失败样例下钻（可过滤）",
	Long:  "按 reason/category/tool/model/project/session 六维过滤，返回具体的失败样例（含内容预览）与命中统计。回答「给我看具体的失败长什么样」。",
	Examples: []string{
		"cc-insights why -p 7d --reason error_text -n 20 -j",
		"cc-insights why --tool Bash --model sonnet",
	},
	Flags: func(fs *flag.FlagSet, opts *cliOptions) {
		registerCommonAnalysisFlags(fs, opts)
		fs.IntVar(&opts.Samples, "samples", opts.Samples, "失败样例数量")
		fs.StringVar(&opts.Reason, "reason", "", "按失败 reason 过滤")
		fs.StringVar(&opts.Category, "category", "", "按失败 category 过滤")
		fs.StringVar(&opts.Tool, "tool", "", "按工具名过滤")
		fs.StringVar(&opts.Model, "model", "", "按模型名过滤")
		fs.StringVar(&opts.Project, "project", "", "按项目路径片段过滤")
		fs.StringVar(&opts.Session, "session", "", "按 session_id 过滤")
	},
	Run: fastRun(func(data *DashboardData, opts cliOptions) error {
		return outputCLI(buildCLIInspectFailuresReport(data, opts), opts.Format, os.Stdout)
	}),
}

var cmdTok = &Command{
	Name:     "tok",
	Short:    "Token 与成本分布",
	Long:     "按模型、项目、会话拆解 Token 消耗与成本，定位最烧 Token 的维度。",
	Examples: []string{"cc-insights tok -p 30d -j"},
	Flags:    registerCommonAnalysisFlags,
	Run: fastRun(func(data *DashboardData, opts cliOptions) error {
		return outputCLI(buildCLICostReport(data, opts.Limit), opts.Format, os.Stdout)
	}),
}

var cmdCmd = &Command{
	Name:     "cmd",
	Short:    "Bash 命令族与高风险命令",
	Long:     "按命令族（family）和具体命令统计调用，识别高风险命令；支持多段 &&/; 链式命令的独立统计与逐段风险检测。",
	Examples: []string{"cc-insights cmd -p 30d -j"},
	Flags:    registerCommonAnalysisFlags,
	Run: fastRun(func(data *DashboardData, opts cliOptions) error {
		return outputCLI(buildCLICommandReport(data, opts.Limit), opts.Format, os.Stdout)
	}),
}

var cmdSes = &Command{
	Name:  "ses",
	Short: "会话生命周期",
	Long:  "会话维度下钻：长会话、高失败会话、Plan/Task 信号、队列操作等。可按 session/project 过滤聚焦具体会话。",
	Examples: []string{
		"cc-insights ses -p 30d -j",
		"cc-insights ses --session SESSION_ID",
	},
	Flags: func(fs *flag.FlagSet, opts *cliOptions) {
		registerCommonAnalysisFlags(fs, opts)
		fs.IntVar(&opts.Samples, "samples", opts.Samples, "失败样例数量")
		fs.StringVar(&opts.Project, "project", "", "按项目路径片段过滤")
		fs.StringVar(&opts.Session, "session", "", "按 session_id 过滤")
	},
	Run: fastRun(func(data *DashboardData, opts cliOptions) error {
		return outputCLI(buildCLISessionReport(data, opts), opts.Format, os.Stdout)
	}),
}

var cmdRec = &Command{
	Name:  "rec",
	Short: "诊断建议与优化方向",
	Long:  "基于证据的诊断：触发条件、根因候选、建议动作和下钻命令。--detail 展开触发条件/根因/优化目标；--id 查看单条诊断；--prompts 分析用户提示词画像与协作偏好。",
	Examples: []string{
		"cc-insights rec -p 30d --detail",
		"cc-insights rec -p 30d --prompts",
		"cc-insights rec --id performance.slowest_call -j",
	},
	Flags: func(fs *flag.FlagSet, opts *cliOptions) {
		registerCommonAnalysisFlags(fs, opts)
		fs.StringVar(&opts.ID, "id", "", "按诊断 ID 过滤")
		fs.BoolVar(&opts.Detail, "detail", false, "展开诊断触发条件、根因候选和优化目标")
		fs.BoolVar(&opts.Prompts, "prompts", false, "分析用户输入提示词画像")
		fs.StringVar(&opts.Project, "project", "", "按项目路径片段过滤（配合 --prompts）")
		fs.StringVar(&opts.Session, "session", "", "按 session_id 过滤（配合 --prompts）")
	},
	Run: func(opts cliOptions) error {
		tf, preset, err := timeFilterFromCLIOptions(opts)
		if err != nil {
			return err
		}
		startedAt := time.Now()
		prepareStartedAt := time.Now()
		if err := prepareCLIDataForRecommendations(); err != nil {
			return err
		}
		prepareDuration := time.Since(prepareStartedAt)
		defer CloseLogger()
		if opts.Prompts {
			reportStartedAt := time.Now()
			report, err := buildCLIPromptReport(tf, preset, opts)
			if err != nil {
				return err
			}
			report.Runtime = &cliRuntimeInfo{
				Source:                   "raw-jsonl",
				PrepareDurationMs:        prepareDuration.Milliseconds(),
				RecommendationDurationMs: time.Since(reportStartedAt).Milliseconds(),
				TotalDurationMs:          time.Since(startedAt).Milliseconds(),
			}
			return outputCLI(report, opts.Format, os.Stdout)
		}
		dataStartedAt := time.Now()
		data, source, err := buildRecommendationDashboardData(tf, preset)
		if err != nil {
			return err
		}
		dataDuration := time.Since(dataStartedAt)
		reportStartedAt := time.Now()
		report := buildCLIRecommendationReport(data, opts)
		recommendationDuration := time.Since(reportStartedAt)
		report.Runtime = &cliRuntimeInfo{
			Source:                   source,
			PrepareDurationMs:        prepareDuration.Milliseconds(),
			DataDurationMs:           dataDuration.Milliseconds(),
			RecommendationDurationMs: recommendationDuration.Milliseconds(),
			TotalDurationMs:          time.Since(startedAt).Milliseconds(),
		}
		return outputCLI(report, opts.Format, os.Stdout)
	},
}

var cmdWeb = &Command{
	Name:  "web",
	Short: "启动本地 Dashboard",
	Long: "启动 Web 仪表盘，统一时间范围驱动概览/失败/命令/Token/工具/会话分析。\n\n" +
		"交互能力：\n" +
		"  · 按模型、项目、会话、失败原因、工具筛选\n" +
		"  · 点击模型柱状图下钻到该模型的命令、文件操作、Agent、会话数据\n" +
		"  · 时间轴滑动窗口、诊断建议联动下钻条件\n\n" +
		"默认监听 :8932，启动后访问 /dashboard。",
	Examples: []string{"cc-insights web --addr :8932"},
	Flags:    nil, // web 仅使用 Config flag（--data/--cache/--addr/--base/--rules）
	Run: func(opts cliOptions) error {
		return runWebServer()
	},
}

// configFlagSet 构造一个仅注册 Config flag 的 FlagSet，供全局帮助复用。
func configFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	var c Config
	registerConfigFlags(fs, &c)
	return fs
}

func printGlobalHelp(w io.Writer) {
	fmt.Fprintln(w, `cc-insights 分析 Claude Code 的使用数据，把历史会话整理成可解释的证据、判断和改进方向。

用法:
  cc-insights [命令] [参数]
  cc-insights <命令> -h    查看该命令的参数与示例
  不带命令等价于 cc-insights sum

命令:`)
	for _, c := range commands {
		fmt.Fprintf(w, "  %-5s %s\n", c.Name, c.Short)
	}
	fmt.Fprintln(w, `
我想看…
  · 整体用法快照              → sum
  · 失败多少、最常见原因      → err
  · 具体某个失败长什么样      → why
  · 哪个模型/项目最烧 Token   → tok
  · 哪些命令用得多、有风险    → cmd
  · 哪些会话太长/失败多       → ses
  · 怎么改 CLAUDE.md/hooks   → rec
  · 交互式看图、按模型下钻    → web

全局参数（所有命令通用）:`)
	writeFlagSetHelp(w, configFlagSet())
}

func printCommandHelp(cmd *Command, fs *flag.FlagSet, w io.Writer) {
	fmt.Fprintf(w, "cc-insights %s — %s\n\n", cmd.Name, cmd.Short)
	if cmd.Long != "" {
		fmt.Fprintln(w, cmd.Long)
		fmt.Fprintln(w)
	}
	if len(cmd.Examples) > 0 {
		fmt.Fprintln(w, "示例:")
		for _, ex := range cmd.Examples {
			fmt.Fprintf(w, "  %s\n", ex)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "参数:")
	writeFlagSetHelp(w, fs)
}

// writeFlagSetHelp 按 flag 名字典序输出 flag 及其说明。
func writeFlagSetHelp(w io.Writer, fs *flag.FlagSet) {
	fs.VisitAll(func(f *flag.Flag) {
		prefix := "-"
		if len(f.Name) > 1 {
			prefix = "--"
		}
		fmt.Fprintf(w, "  %-16s %s\n", prefix+f.Name, f.Usage)
	})
}
