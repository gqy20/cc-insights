package main

import (
	"bytes"
	"encoding/json"
	"strings"
)

// --- chain command analysis: 多段 && 链式命令解析 ---

// splitChainSegments 将单行命令按 && 和 ; 分割为多个段，引号内的 && 和 ; 不会被分割。
// 例如：cd /tmp && pnpm test → ["cd /tmp", "pnpm test"]
//
//	echo "a&&b"          → ["echo \"a&&b\""] (引号内不拆)
//	grep "x;y" file      → ["grep \"x;y\" file"] (引号内不拆)
func splitChainSegments(line string) []string {
	var segments []string
	var current strings.Builder
	inSingle, inDouble := false, false

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
			current.WriteByte(c)
		case c == '"' && !inSingle:
			inDouble = !inDouble
			current.WriteByte(c)
		case !inSingle && !inDouble:
			if i+1 < len(line) && line[i] == '&' && line[i+1] == '&' {
				segments = append(segments, strings.TrimSpace(current.String()))
				current.Reset()
				i++ // 跳过第二个 &
			} else if c == ';' {
				segments = append(segments, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteByte(c)
			}
		default:
			current.WriteByte(c)
		}
	}
	if tail := strings.TrimSpace(current.String()); tail != "" {
		segments = append(segments, tail)
	}
	return segments
}

// extractShellSegments 从完整命令字符串中提取所有可执行段。
// 处理多行（只取第一个非空非注释行）、注释跳过、&&/; 链式分割。
// 返回的每个元素都是经过 trim 的独立命令段。
func extractShellSegments(command string) []string {
	for _, line := range strings.Split(command, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := splitChainSegments(line)
		var result []string
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				result = append(result, p)
			}
		}
		return result // 只取第一个非空行（保持原有行为）
	}
	return nil
}

// --- end chain command analysis ---

func recordStructuredToolInputLocked(agg *ProjectAggregate, call *pendingToolCall) {
	switch call.Tool {
	case "Bash":
		var input struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(call.Input, &input); err != nil || strings.TrimSpace(input.Command) == "" {
			return
		}
		segments := extractShellSegments(input.Command)
		if len(segments) == 0 {
			return
		}
		// 主命令名 = 链首段（ChainCommands 为全部分段，CommandName 为主命令）
		primaryName := bashCommandName(segments[0])
		call.CommandName = primaryName

		// 记录链中每个唯一命令
		recorded := make(map[string]bool)
		for _, seg := range segments {
			cmdName := bashCommandName(seg)
			if recorded[cmdName] {
				continue // 同链去重：echo && echo 只计 1 次
			}
			recorded[cmdName] = true

			stat := ensureBashCommandStat(agg, cmdName)
			stat.CallCount++
			ensureBashCommandModelStat(agg, cmdName, call.Model).CallCount++
			if stat.SampleCommand == "" {
				stat.SampleCommand = previewString(input.Command, 180)
			}

			// 逐段风险检测
			level, reason := classifyBashRisk(seg)
			if level != "" && (stat.RiskLevel == "" || riskRank(level) > riskRank(stat.RiskLevel)) {
				stat.RiskLevel = level
				stat.RiskReason = reason
			}
		}

		// 存储链中所有命令名（用于结果分发）
		call.ChainCommands = make([]string, 0, len(recorded))
		for name := range recorded {
			call.ChainCommands = append(call.ChainCommands, name)
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
		ensureFileOperationModelStat(agg, call.Tool, path, call.Model).CallCount++
		// file_analysis: 按路径聚合（跨操作类型）
		recordFileHotStatLocked(agg, call.Tool, path)
	}
}

// ensureBashCommandModelStat 取得或创建按模型归因的 Bash 命令统计。
func ensureBashCommandModelStat(agg *ProjectAggregate, commandName, model string) *BashCommandModelStat {
	if agg.BashCommandModelStats == nil {
		agg.BashCommandModelStats = make(map[string]*BashCommandModelStat)
	}
	if commandName == "" {
		commandName = "unknown"
	}
	if model == "" {
		model = "unknown"
	}
	key := model + "\x00" + commandName
	if agg.BashCommandModelStats[key] == nil {
		agg.BashCommandModelStats[key] = &BashCommandModelStat{Model: model, CommandName: commandName}
	}
	return agg.BashCommandModelStats[key]
}

// ensureFileOperationModelStat 取得或创建按模型归因的文件操作统计。
func ensureFileOperationModelStat(agg *ProjectAggregate, operation, path, model string) *FileOperationModelStat {
	if agg.FileOperationModelStats == nil {
		agg.FileOperationModelStats = make(map[string]*FileOperationModelStat)
	}
	if model == "" {
		model = "unknown"
	}
	key := model + "\x00" + operation + "\x00" + path
	if agg.FileOperationModelStats[key] == nil {
		agg.FileOperationModelStats[key] = &FileOperationModelStat{Model: model, Operation: operation, Path: path}
	}
	return agg.FileOperationModelStats[key]
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

// updateBashResultStat 更新单个 BashCommandStat 的成功/失败/missing 计数。
func updateBashResultStat(stat *BashCommandStat, failed, missing bool) {
	switch {
	case missing:
		stat.MissingResultCount++
	case failed:
		stat.FailureCount++
	default:
		stat.SuccessCount++
	}
}

// applyBashCommandModelResult 更新按模型归因的 Bash 命令结果计数。
func applyBashCommandModelResult(stat *BashCommandModelStat, failed, missing bool) {
	if stat == nil {
		return
	}
	switch {
	case missing:
		stat.MissingResultCount++
	case failed:
		stat.FailureCount++
	default:
		stat.SuccessCount++
	}
}

func addCommandOrFileResultLocked(agg *ProjectAggregate, call pendingToolCall, failed bool, missing bool) {
	if call.CommandName != "" {
		// 主命令
		updateBashResultStat(ensureBashCommandStat(agg, call.CommandName), failed, missing)
		applyBashCommandModelResult(ensureBashCommandModelStat(agg, call.CommandName, call.Model), failed, missing)
		// 链中其他命令：结果全局分发（整个链共享同一成败状态）
		for _, name := range call.ChainCommands {
			if name == call.CommandName {
				continue
			}
			updateBashResultStat(ensureBashCommandStat(agg, name), failed, missing)
			applyBashCommandModelResult(ensureBashCommandModelStat(agg, name, call.Model), failed, missing)
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
		if mStat := ensureFileOperationModelStat(agg, stat.Operation, stat.Path, call.Model); mStat != nil {
			switch {
			case missing:
				mStat.MissingResultCount++
			case failed:
				mStat.FailureCount++
			default:
				mStat.SuccessCount++
			}
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
	// 遍历所有段（而非仅第一段），检测任意段中的危险命令
	for _, seg := range extractShellSegments(command) {
		lower := strings.ToLower(seg)
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
		}
	}
	return "", ""
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
	// 安全化：替换引号和控制字符，防止破坏外层 JSON
	value = strings.ReplaceAll(value, `"`, "'")
	value = strings.ReplaceAll(value, "\\", "/")
	var sb strings.Builder
	sb.Grow(len(value))
	for _, r := range value {
		if r >= 32 && r != 127 {
			sb.WriteRune(r)
		}
	}
	value = sb.String()
	if len(value) > limit {
		return value[:limit]
	}
	return value
}
