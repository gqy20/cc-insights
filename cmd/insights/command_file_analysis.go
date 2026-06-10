package main

import (
	"bytes"
	"encoding/json"
	"strings"
)

func recordStructuredToolInputLocked(agg *ProjectAggregate, call *pendingToolCall) {
	switch call.Tool {
	case "Bash":
		var input struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(call.Input, &input); err != nil || strings.TrimSpace(input.Command) == "" {
			return
		}
		commandName := bashCommandName(input.Command)
		riskLevel, riskReason := classifyBashRisk(input.Command)
		call.CommandName = commandName
		stat := ensureBashCommandStat(agg, commandName)
		stat.CallCount++
		if stat.SampleCommand == "" {
			stat.SampleCommand = previewString(input.Command, 180)
		}
		if riskLevel != "" {
			stat.RiskLevel = riskLevel
			stat.RiskReason = riskReason
		}
	case "Read", "Edit", "Write", "MultiEdit":
		path := filePathFromToolInput(call.Input)
		if path == "" {
			return
		}
		key := call.Tool + "\x00" + path
		call.FileOpKey = key
		if agg.FileOperationStats[key] == nil {
			agg.FileOperationStats[key] = &FileOperationStat{Operation: call.Tool, Path: path}
		}
		agg.FileOperationStats[key].CallCount++
		// file_analysis: 按路径聚合（跨操作类型）
		recordFileHotStatLocked(agg, call.Tool, path)
	}
}

// --- file_analysis: 按路径聚合的文件活跃度统计 ---

func ensureFileHotStat(agg *ProjectAggregate, path string) *FileHotStat {
	if agg.FileHotStats == nil {
		agg.FileHotStats = make(map[string]*FileHotStat)
	}
	if agg.FileHotStats[path] == nil {
		agg.FileHotStats[path] = &FileHotStat{Path: path}
	}
	return agg.FileHotStats[path]
}

func recordFileHotStatLocked(agg *ProjectAggregate, tool, path string) {
	stat := ensureFileHotStat(agg, path)
	switch tool {
	case "Read":
		stat.ReadCount++
	case "Edit", "MultiEdit":
		stat.EditCount++
	case "Write":
		stat.WriteCount++
	}
}

func updateFileHotResultLocked(agg *ProjectAggregate, path string, failed, missing bool) {
	stat := ensureFileHotStat(agg, path)
	switch {
	case missing:
		stat.MissingCount++
	case failed:
		stat.FailureCount++
	default:
		stat.SuccessCount++
	}
}

// --- end file_analysis ---

func addCommandOrFileResultLocked(agg *ProjectAggregate, call pendingToolCall, failed bool, missing bool) {
	if call.CommandName != "" {
		stat := ensureBashCommandStat(agg, call.CommandName)
		switch {
		case missing:
			stat.MissingResultCount++
		case failed:
			stat.FailureCount++
		default:
			stat.SuccessCount++
		}
	}
	if call.FileOpKey != "" && agg.FileOperationStats[call.FileOpKey] != nil {
		stat := agg.FileOperationStats[call.FileOpKey]
		switch {
		case missing:
			stat.MissingResultCount++
		case failed:
			stat.FailureCount++
		default:
			stat.SuccessCount++
		}
	}
	// file_analysis: 同步更新按路径聚合的统计
	if call.FileOpKey != "" {
		if parts := strings.SplitN(call.FileOpKey, "\x00", 2); len(parts) == 2 {
			updateFileHotResultLocked(agg, parts[1], failed, missing)
		}
	}
}

func ensureBashCommandStat(agg *ProjectAggregate, commandName string) *BashCommandStat {
	if agg.BashCommandStats == nil {
		agg.BashCommandStats = make(map[string]*BashCommandStat)
	}
	if commandName == "" {
		commandName = "unknown"
	}
	if agg.BashCommandStats[commandName] == nil {
		agg.BashCommandStats[commandName] = &BashCommandStat{CommandName: commandName}
	}
	return agg.BashCommandStats[commandName]
}

func bashCommandName(command string) string {
	fields := strings.Fields(firstExecutableShellSegment(command))
	if len(fields) == 0 {
		return "unknown"
	}
	if fields[0] == "sudo" && len(fields) > 1 {
		return "sudo " + fields[1]
	}
	return fields[0]
}

func classifyBashRisk(command string) (string, string) {
	lower := strings.ToLower(firstExecutableShellSegment(command))
	switch {
	case strings.HasPrefix(lower, "rm -rf ") || strings.HasPrefix(lower, "rm -fr ") || lower == "rm -rf" || lower == "rm -fr":
		return "high", "recursive delete"
	case strings.HasPrefix(lower, "git reset --hard") || strings.HasPrefix(lower, "git clean -fd"):
		return "high", "destructive git cleanup"
	case strings.HasPrefix(lower, "sudo "):
		return "medium", "privileged command"
	case strings.HasPrefix(lower, "curl ") && strings.Contains(lower, "| sh"):
		return "high", "download pipe to shell"
	case strings.HasPrefix(lower, "wget ") && strings.Contains(lower, "| sh"):
		return "high", "download pipe to shell"
	default:
		return "", ""
	}
}

func firstExecutableShellSegment(command string) string {
	for _, segment := range strings.Split(command, "\n") {
		segment = strings.TrimSpace(segment)
		if segment == "" || strings.HasPrefix(segment, "#") {
			continue
		}
		for _, separator := range []string{"&&", ";"} {
			if idx := strings.Index(segment, separator); idx >= 0 {
				segment = strings.TrimSpace(segment[:idx])
			}
		}
		return segment
	}
	return ""
}

func filePathFromToolInput(raw json.RawMessage) string {
	var input struct {
		FilePath string `json:"file_path"`
		Path     string `json:"path"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return ""
	}
	if input.FilePath != "" {
		return input.FilePath
	}
	return input.Path
}

func rawStringField(raw json.RawMessage, field string) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return ""
	}
	value, ok := data[field]
	if !ok {
		return ""
	}
	s, _ := value.(string)
	return s
}

func rawJSONText(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

func previewString(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > limit {
		return value[:limit]
	}
	return value
}
