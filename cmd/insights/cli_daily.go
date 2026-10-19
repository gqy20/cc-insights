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
//   - 每设备每次导出为全量快照，消费端按 (device, provider, date) 取最新
//     再按 (provider, date) 求和，重复导出天然幂等
//   - schema 只加字段不改名，语义变更升 v

const dailyDigestSchemaVersion = 1

// dailyDigestRow 每日×渠道消耗行
type dailyDigestRow struct {
	V          int                       `json:"v"`
	Device     string                    `json:"device"`
	Provider   string                    `json:"provider"`
	Date       string                    `json:"date"`
	TZ         string                    `json:"tz"`
	Messages   int                       `json:"messages"`
	Sessions   int                       `json:"sessions"`
	ToolCalls  int                       `json:"tool_calls"`
	Tokens     int                       `json:"tokens"`
	Models     map[string]dailyModelStat `json:"models,omitempty"`
	ExportedAt string                    `json:"exported_at"`
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

// writeDailyDigestFile 原子写 JSONL 快照（tmp+rename，消费端随时读到完整文件）。
func writeDailyDigestFile(path string, rows []dailyDigestRow) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建导出目录失败: %w", err)
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
		if err := writeDailyDigestFile(out, rows); err != nil {
			return err
		}
		Info("daily digest 已写入", "path", out, "rows", len(rows), "device", device)
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
