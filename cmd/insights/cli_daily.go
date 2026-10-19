package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 每日×渠道消耗导出（sum --daily）。
//
// 交换格式约定（跨设备合并，详见 docs）：
//   - 自描述 JSONL，每行 = 该设备该渠道该天的总量计数器
//   - token 为源数据，成本由汇聚端按统一定价现算（各设备 pricing 可能不一致）
//   - 默认按键合并写入：键 = (device, provider, date)，本次导出的行覆盖旧行
//     （计数口径修正、当天补录生效），源数据已消失的旧行原样保留——文件成为
//     只增不减的系统记录；--rewrite 回退为全量快照重写
//   - 消费端按 (device, provider, date) 取最新再按 (provider, date) 求和，
//     合并模式下文件内键已唯一，直接求和即可
//   - schema 只加字段不改名，语义变更升 v

const dailyDigestSchemaVersion = 2

// dailyDigestRow 每日×渠道消耗行。
// v2 新增：failures/suspected/missing、failure_reasons、hourly、projects。
type dailyDigestRow struct {
	V         int                       `json:"v"`
	Device    string                    `json:"device"`
	Provider  string                    `json:"provider"`
	Date      string                    `json:"date"`
	TZ        string                    `json:"tz"`
	Messages  int                       `json:"messages"`
	Sessions  int                       `json:"sessions"`
	ToolCalls int                       `json:"tool_calls"`
	Tokens    int                       `json:"tokens"`
	Models    map[string]dailyModelStat `json:"models,omitempty"`
	// 质量：确认失败 / 疑似（仅文本命中，不计入失败率）/ 无结果
	Failures  int `json:"failures"`
	Suspected int `json:"suspected"`
	Missing   int `json:"missing"`
	// 失败原因分布：category/reason -> 次数（不含 suspect_ 前缀项）
	Reasons map[string]int `json:"failure_reasons,omitempty"`
	// 活跃时段：0-23 时消息数
	Hourly [24]int `json:"hourly"`
	// 项目分布：当天各项目消息数与 token
	Projects   map[string]dailyProjectStat `json:"projects,omitempty"`
	ExportedAt string                      `json:"exported_at"`
}

type dailyProjectStat struct {
	Messages int `json:"messages"`
	Tokens   int `json:"tokens"`
}

type dailyModelStat struct {
	Count  int `json:"count"`
	Tokens int `json:"tokens"`
}

// digestDeviceName 导出设备名：--device 优先，默认 hostname。
func digestDeviceName(override string) string {
	if override != "" {
		return override
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "unknown-device"
	}
	return host
}

// buildDailyDigestRows 逐渠道生成日表行。
// provider=all 时逐渠道出行（保留 provider 身份，合并交给消费端）；
// 其余渠道出单渠道行。
func buildDailyDigestRows(device string) ([]dailyDigestRow, error) {
	providers := []Provider{resolveProvider(cfg.Provider, cfg.DataDir)}
	isAll := providers[0] == ProviderAll
	if isAll {
		if cfg.DataDirExplicit {
			return nil, fmt.Errorf("--provider all 不支持自定义 --data（各渠道使用各自默认数据目录）")
		}
		providers = allMergeProviderList
	}

	now := time.Now()
	exportedAt := now.Format(time.RFC3339)
	tz := now.Format("-07:00")
	var rows []dailyDigestRow
	for _, provider := range providers {
		sub := cfg
		sub.Provider = provider
		if isAll {
			sub.DataDir = defaultDataDirForProvider(provider)
			sub.DataDirExplicit = false
		}
		cache, err := refreshCacheForConfig(sub, false)
		if err != nil {
			Warn("daily digest 跳过 provider", "provider", string(provider), "error", err.Error())
			continue
		}
		for date, day := range cache.DailyStats {
			row := dailyDigestRow{
				V: dailyDigestSchemaVersion, Device: device, Provider: string(provider),
				Date: date, TZ: tz, Messages: day.MessageCount, Sessions: day.SessionCount,
				ToolCalls: day.ToolCallCount, ExportedAt: exportedAt,
			}
			if len(day.ModelCounts) > 0 {
				row.Models = make(map[string]dailyModelStat, len(day.ModelCounts))
				for model, count := range day.ModelCounts {
					row.Models[model] = dailyModelStat{Count: count, Tokens: day.ModelTokens[model]}
				}
			}
			for _, tokens := range day.ModelTokens {
				row.Tokens += tokens
			}
			row.Hourly = day.HourlyCounts
			if len(day.ProjectCounts) > 0 {
				row.Projects = make(map[string]dailyProjectStat, len(day.ProjectCounts))
				for project, messages := range day.ProjectCounts {
					row.Projects[project] = dailyProjectStat{Messages: messages}
				}
			}
			// 质量：日级 runtime 快照的失败/疑似/missing 与原因分布
			if runtime, ok := cache.DailyRuntime[date]; ok {
				for _, toolStat := range runtime.ToolStats {
					row.Failures += toolStat.FailureCount
					row.Missing += toolStat.MissingResultCount
				}
				for _, stat := range runtime.FailureReasons {
					if isSuspectedFailureReason(stat.Reason) {
						row.Suspected += stat.Count
						continue
					}
					if row.Reasons == nil {
						row.Reasons = make(map[string]int)
					}
					row.Reasons[stat.Category+"/"+stat.Reason] += stat.Count
				}
			}
			// 各项目当日 token（日级项目 runtime 的模型用量求和）
			if projects, ok := cache.DailyProjectRuntime[date]; ok && len(row.Projects) > 0 {
				for project, snapshot := range projects {
					if _, tracked := row.Projects[project]; !tracked {
						continue
					}
					stat := row.Projects[project]
					for _, usage := range snapshot.ModelUsage {
						stat.Tokens += usage.Tokens
					}
					row.Projects[project] = stat
				}
			}
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("没有可导出的数据（各渠道缓存为空或不可用）")
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date < rows[j].Date
		}
		return rows[i].Provider < rows[j].Provider
	})
	return rows, nil
}

// dailyDigestKey 合并键：同设备同渠道同一天只保留一行。
type dailyDigestKey struct {
	Device   string
	Provider string
	Date     string
}

func dailyDigestKeyOf(row dailyDigestRow) dailyDigestKey {
	return dailyDigestKey{Device: row.Device, Provider: row.Provider, Date: row.Date}
}

// readDailyDigestRows 容错读取既有导出文件：损坏行跳过并告警，不整体失败。
func readDailyDigestRows(path string) ([]dailyDigestRow, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var rows []dailyDigestRow
	scanner := newJSONLScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		var row dailyDigestRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			Warn("daily digest 旧行解析失败，跳过", "path", path, "line", line)
			continue
		}
		if row.Date == "" || row.Provider == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows, scanner.Err()
}

// mergeDailyDigestRows 合并语义：
//   - 键 = (device, provider, date)，文件内每键一行
//   - 本次导出的行覆盖旧行（exported_at 更新，口径修正/当天补录生效）
//   - 源数据已消失的旧行原样保留（文件只增不减）
//   - 冲突时 exported_at 新者胜（防御旧文件时钟倒拨）
func mergeDailyDigestRows(existing, fresh []dailyDigestRow) []dailyDigestRow {
	merged := make(map[dailyDigestKey]dailyDigestRow, len(existing)+len(fresh))
	for _, row := range existing {
		merged[dailyDigestKeyOf(row)] = row
	}
	for _, row := range fresh {
		key := dailyDigestKeyOf(row)
		if old, ok := merged[key]; ok && old.ExportedAt > row.ExportedAt {
			continue
		}
		merged[key] = row
	}
	out := make([]dailyDigestRow, 0, len(merged))
	for _, row := range merged {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Device != out[j].Device {
			return out[i].Device < out[j].Device
		}
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].Provider < out[j].Provider
	})
	return out
}

// writeDailyDigestFile 按键合并后原子写 JSONL（tmp+rename，消费端随时读到完整文件）。
// rewrite=true 时跳过合并，退回全量快照重写：以当前源数据为准，清掉幽灵行。
func writeDailyDigestFile(path string, rows []dailyDigestRow, rewrite bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建导出目录失败: %w", err)
	}
	if !rewrite {
		existing, err := readDailyDigestRows(path)
		if err != nil {
			return fmt.Errorf("读取既有导出失败: %w", err)
		}
		if len(existing) > 0 {
			rows = mergeDailyDigestRows(existing, rows)
		}
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("写导出文件失败: %w", err)
	}
	enc := json.NewEncoder(f)
	for _, row := range rows {
		if err := enc.Encode(row); err != nil {
			f.Close()
			os.Remove(tmp)
			return fmt.Errorf("编码导出行失败: %w", err)
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("写导出文件失败: %w", err)
	}
	return os.Rename(tmp, path)
}

// runDailyDigest sum --daily 入口：单次导出或 --watch 周期刷新。
func runDailyDigest(opts cliOptions) error {
	device := digestDeviceName(opts.Device)
	writeOnce := func() error {
		rows, err := buildDailyDigestRows(device)
		if err != nil {
			return err
		}
		out := opts.Out
		if out == "" {
			// 默认落 cc-insights 目录，其他项目从固定位置读取
			out = filepath.Join(filepath.Dir(cfg.CacheDir), "digest", "daily-"+sanitizeFileName(device)+".jsonl")
		}
		if out == "-" {
			enc := json.NewEncoder(os.Stdout)
			for _, row := range rows {
				if err := enc.Encode(row); err != nil {
					return err
				}
			}
			return nil
		}
		if err := writeDailyDigestFile(out, rows, opts.Rewrite); err != nil {
			return err
		}
		Info("daily digest 已写入", "path", out, "rows", len(rows), "device", device, "merge", !opts.Rewrite)
		if opts.Watch <= 0 {
			// 单次模式向 stdout 报告位置，方便脚本/人取用
			fmt.Fprintf(os.Stderr, "daily digest: %s (%d rows, device=%s)\n", out, len(rows), device)
		}
		return nil
	}

	if opts.Watch <= 0 {
		return writeOnce()
	}
	// 实时模式：增量缓存（tail/文件级复用）让每次刷新秒级完成
	Info("daily digest watch 启动", "interval", opts.Watch.String(), "out", opts.Out, "device", device)
	if err := writeOnce(); err != nil {
		Warn("daily digest 首次导出失败", "error", err.Error())
	}
	ticker := time.NewTicker(opts.Watch)
	defer ticker.Stop()
	for range ticker.C {
		if err := writeOnce(); err != nil {
			Warn("daily digest 刷新失败", "error", err.Error())
		}
	}
	return nil
}

// sanitizeFileName 设备名可能含 / 等字符，文件名需归一。
func sanitizeFileName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "unknown-device"
	}
	return b.String()
}
