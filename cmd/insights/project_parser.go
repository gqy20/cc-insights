package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"
)

// ParseProjectsConcurrentOnce 一次遍历并发解析所有项目统计
// 这个函数将所有统计合并到一次遍历中，大幅提升性能
func ParseProjectsConcurrentOnce(tf TimeFilter) (*ProjectAggregate, error) {
	return ParseProjectsConcurrentOnceFromDir(tf, cfg.DataDir)
}

// ParseProjectsConcurrentOnceFromDir 一次遍历并发解析指定数据目录下的项目统计
func ParseProjectsConcurrentOnceFromDir(tf TimeFilter, dataDir string) (*ProjectAggregate, error) {
	provider := resolveProvider(cfg.Provider, dataDir)
	fileInfos, err := listLogFileInfos(provider, dataDir)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 日志目录失败: %w", provider, err)
	}

	aggregate := newProjectAggregate()
	files := make([]string, 0, len(fileInfos))
	for _, info := range fileInfos {
		files = append(files, info.AbsPath)
	}

	maxWorkers := runtime.NumCPU()
	if len(files) < maxWorkers {
		maxWorkers = len(files)
	}
	if maxWorkers == 0 {
		aggregate.finalize()
		return aggregate, nil
	}

	jobs := make(chan string, maxWorkers*2)
	results := make(chan *ProjectAggregate, maxWorkers)
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			workerAggregate := newProjectAggregate()
			for filePath := range jobs {
				parseLogFileAggregate(provider, filePath, tf, workerAggregate)
			}
			results <- workerAggregate
		}()
	}

	for _, filePath := range files {
		jobs <- filePath
	}
	close(jobs)
	go func() {
		wg.Wait()
		close(results)
	}()

	for fileAggregate := range results {
		mergeProjectAggregate(aggregate, fileAggregate)
	}

	if provider == ProviderClaude || provider == ProviderQoder {
		recordInstalledSkillsLocked(aggregate, dataDir)
	}
	if provider == ProviderCodex {
		if err := recordCodexSQLiteHooksLocked(aggregate, dataDir, tf); err != nil {
			Warn("Codex hook SQLite 扫描失败", "error", err.Error())
		}
	}
	if provider == ProviderQoder {
		if dbPath := qoderStateDBPath(); dbPath != "" {
			if err := recordQoderStateDBLocked(aggregate, dbPath, tf); err != nil {
				Warn("Qoder 状态库扫描失败", "path", dbPath, "error", err.Error())
			}
			if err := recordQoderContextCategories(aggregate, dbPath, tf); err != nil {
				Warn("Qoder 上下文构成读取失败", "path", dbPath, "error", err.Error())
			}
		}
	}

	// 后处理：生成输出格式数据
	aggregate.finalize()

	return aggregate, nil
}

// parseProjectFileAggregate 解析单个项目文件并更新聚合数据
func parseProjectFileAggregate(filePath string, tf TimeFilter, agg *ProjectAggregate) {
	parseProjectFileAggregateWithAgentName(filePath, tf, agg, "")
}

// parseProjectFileAggregateWithAgentName 额外接受一个子代理可读名字：Qoder 不为子代理
// 写 Claude 那种 agent-name 记录，名字只能由调用方从 subagents/*.meta.json 取。
func parseProjectFileAggregateWithAgentName(filePath string, tf TimeFilter, agg *ProjectAggregate, agentName string) {
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()

	pendingTools := make(map[string]pendingToolCall)
	sessionActiveSkills := make(map[string][]string)
	lastMsgTs := make(map[string]time.Time) // sessionID -> 上一条带 timestamp 的 message 行时间，用于算单请求 round-trip
	// 一次 API 响应的用量只记一次（见 shouldAccountResponse）。
	accountedResponses := make(map[string]bool)
	// 子代理文件里出现过的 agentId，解析结束后统一补可读名字。
	seenAgentIDs := make(map[string]bool)
	scanner := newJSONLScanner(f)
	for scanner.Scan() {
		var record ProjectRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue // 跳过损坏/非法 JSON 行
		}
		if agentName != "" && record.AgentID != "" {
			seenAgentIDs[record.AgentID] = true
		}

		timestamp, hasTimestamp := parseProjectRecordTimestamp(record.Timestamp)
		if hasTimestamp && !tf.Contains(timestamp) {
			continue
		}
		if !hasTimestamp && hasTimeFilter(tf) {
			continue
		}

		projectName := record.Cwd
		if projectName == "" {
			projectName = "Unknown"
		}

		recordRuntimeEventLocked(agg, record, timestamp, projectName)
		if hasTimestamp {
			recordRuntimeEventLocked(ensureDailyRuntimeAggregate(agg, timestamp.Format("2006-01-02")), record, timestamp, projectName)
		}
		if record.Type == "attachment" {
			activeNames, attachmentType := extractAttachmentSkillSignals(record.Attachment)
			if len(activeNames) > 0 && record.SessionID != "" {
				switch attachmentType {
				case "invoked_skills", "dynamic_skill":
					sessionActiveSkills[record.SessionID] = appendUniqueStrings(sessionActiveSkills[record.SessionID], activeNames...)
				}
			}
		}

		// system/compact_boundary：Claude 与 Qoder 都会写，compactMetadata 带压缩前后
		// token、被摘要的消息数与压缩自身耗时。这是权威的压缩证据，不再按上下文比例回落推断。
		if record.Type == "system" && record.Subtype == "compact_boundary" && hasTimestamp {
			recordCompactBoundaryLocked(agg, record, timestamp.Format("2006-01-02"))
			continue
		}

		// system/turn_duration：官方一整轮耗时，按 project/daily/session 四层累积（session 维度
		// 的 SystemDurationMs 已由 recordSessionSystemEvent 维护，这里补充其余维度 + 慢回合 Top-N）。
		if record.Type == "system" && record.Subtype == "turn_duration" && hasTimestamp {
			dur := int64(record.DurationMs)
			msgCount := record.MessageCount
			sid := record.SessionID
			dateKey := timestamp.Format("2006-01-02")
			recordTurnDurationLocked(agg, dur, msgCount, sid, projectName, timestamp)
			recordTurnDurationLocked(ensureDailyRuntimeAggregate(agg, dateKey), dur, msgCount, sid, projectName, timestamp)
			recordTurnDurationLocked(ensureDailyProjectRuntimeAggregate(agg, dateKey, projectName), dur, msgCount, sid, projectName, timestamp)
			recordTurnDurationLocked(ensureDailySessionRuntimeAggregate(agg, dateKey, sid), dur, msgCount, sid, projectName, timestamp)
			continue
		}

		if record.Type == "user" {
			parseToolResults(record, timestamp, projectName, pendingTools, agg)
			if hasTimestamp && record.SessionID != "" {
				lastMsgTs[record.SessionID] = timestamp
			}
			continue
		}

		// 只统计 assistant 消息
		if record.Type != "assistant" {
			continue
		}

		var msg AssistantMessage
		if err := json.Unmarshal(record.Message, &msg); err != nil {
			continue
		}

		// 1. 项目统计
		if agg.ProjectStats[projectName] == nil {
			agg.ProjectStats[projectName] = &ProjectStatItem{
				Project: projectName,
			}
		}
		agg.ProjectStats[projectName].MessageCount++

		// 2. 星期统计
		weekday := int(timestamp.Weekday())  // 0=周日, 1=周一...
		adjustedWeekday := (weekday + 6) % 7 // 转换为0=周一
		agg.WeekdayData[adjustedWeekday].MessageCount++

		// 3. 每日活动
		dateKey := timestamp.Format("2006-01-02")
		agg.DailyActivity[dateKey]++
		if agg.DailyProjectCounts[dateKey] == nil {
			agg.DailyProjectCounts[dateKey] = make(map[string]int)
		}
		agg.DailyProjectCounts[dateKey][projectName]++

		// 3.5 每日会话去重（同一 sessionID 同天只计一次）
		if record.SessionID != "" {
			if agg.DailySessions[dateKey] == nil {
				agg.DailySessions[dateKey] = make(map[string]bool)
			}
			if !agg.DailySessions[dateKey][record.SessionID] {
				agg.DailySessions[dateKey][record.SessionID] = true
			}
		}

		// 4. 小时统计
		hour := timestamp.Hour()
		agg.HourlyCounts[hour]++
		dailyHourlyCounts := agg.DailyHourlyCounts[dateKey]
		dailyHourlyCounts[hour]++
		agg.DailyHourlyCounts[dateKey] = dailyHourlyCounts

		// 5. 模型使用统计
		dailyRuntimeAgg := ensureDailyRuntimeAggregate(agg, dateKey)
		dailyProjectRuntimeAgg := ensureDailyProjectRuntimeAggregate(agg, dateKey, projectName)
		dailySessionRuntimeAgg := ensureDailySessionRuntimeAggregate(agg, dateKey, record.SessionID)
		if msg.Model != "" && shouldAccountResponse(msg.ID, msg.Usage, accountedResponses) {
			tokens := msg.Usage.InputTokens + msg.Usage.OutputTokens
			recordModelUsageLocked(agg, msg.Model, tokens)
			recordModelUsageLocked(dailyRuntimeAgg, msg.Model, tokens)
			recordModelUsageLocked(dailyProjectRuntimeAgg, msg.Model, tokens)
			recordModelUsageLocked(dailySessionRuntimeAgg, msg.Model, tokens)
			for _, target := range []*ProjectAggregate{agg, dailyRuntimeAgg, dailyProjectRuntimeAgg, dailySessionRuntimeAgg} {
				recordModelTokenBreakdownLocked(target, msg.Model, msg.Usage.InputTokens, msg.Usage.CacheReadInputTokens+msg.Usage.CacheCreationInputTokens, msg.Usage.OutputTokens, 0)
			}
			if agg.DailyModelCounts[dateKey] == nil {
				agg.DailyModelCounts[dateKey] = make(map[string]int)
			}
			agg.DailyModelCounts[dateKey][msg.Model]++
			if agg.DailyModelTokens[dateKey] == nil {
				agg.DailyModelTokens[dateKey] = make(map[string]int)
			}
			agg.DailyModelTokens[dateKey][msg.Model] += tokens
			roundTripMs := requestRoundTripMs(lastMsgTs, record.SessionID, timestamp, hasTimestamp)
			recordCostUsageLocked(agg, msg, record, projectName, roundTripMs)
			recordCostUsageLocked(dailyRuntimeAgg, msg, record, projectName, roundTripMs)
			recordCostUsageLocked(dailyProjectRuntimeAgg, msg, record, projectName, roundTripMs)
			recordCostUsageLocked(dailySessionRuntimeAgg, msg, record, projectName, roundTripMs)
		}

		// 6. 工具调用统计
		// 更新 lastMsgTs：作为同一 session 下一条 assistant 的 round-trip 起点（在 cost 统计之后更新）。
		if hasTimestamp && record.SessionID != "" {
			lastMsgTs[record.SessionID] = timestamp
		}
		for _, content := range msg.Content {
			if content.Type != "tool_use" || content.ID == "" || content.Name == "" {
				continue
			}
			pendingTools[content.ID] = pendingToolCall{
				ID:          content.ID,
				Tool:        content.Name,
				Model:       msg.Model,
				Project:     projectName,
				SessionID:   record.SessionID,
				AgentID:     record.AgentID,
				IsSidechain: record.IsSidechain,
				Timestamp:   timestamp,
				Input:       content.Input,
			}
			call := pendingTools[content.ID]
			if content.Name == "Skill" {
				skillName, argsLen := extractSkillInvocation(content.Input)
				call.SkillName = skillName
				call.SkillArgsLength = argsLen
			}
			pendingTools[content.ID] = call
			addToolCallLocked(agg, content.Name, msg.Model)
			addAgentToolCallLocked(agg, call)
			addSessionToolCallLocked(agg, call)
			recordStructuredToolInputLocked(agg, &call)
			if content.Name == "Skill" {
				recordSkillToolUseLocked(agg, call, call.SkillName, call.SkillArgsLength)
				if record.SessionID != "" && call.SkillName != "" {
					sessionActiveSkills[record.SessionID] = appendUniqueStrings(sessionActiveSkills[record.SessionID], call.SkillName)
				}
			} else if record.SessionID != "" {
				call.ChainSkills = appendUniqueStrings(call.ChainSkills, sessionActiveSkills[record.SessionID]...)
				for _, skillName := range sessionActiveSkills[record.SessionID] {
					recordSkillSessionToolLocked(agg, skillName, content.Name, false, false)
				}
			}
			if content.Name != "Skill" && record.AttributionSkill != "" {
				call.ChainSkills = appendUniqueStrings(call.ChainSkills, record.AttributionSkill)
				recordSkillSessionToolLocked(agg, record.AttributionSkill, content.Name, false, false)
			}
			dailyCall := call
			addToolCallLocked(dailyRuntimeAgg, content.Name, msg.Model)
			addAgentToolCallLocked(dailyRuntimeAgg, dailyCall)
			addSessionToolCallLocked(dailyRuntimeAgg, dailyCall)
			recordStructuredToolInputLocked(dailyRuntimeAgg, &dailyCall)
			addToolCallLocked(dailyProjectRuntimeAgg, content.Name, msg.Model)
			addAgentToolCallLocked(dailyProjectRuntimeAgg, dailyCall)
			addSessionToolCallLocked(dailyProjectRuntimeAgg, dailyCall)
			recordStructuredToolInputLocked(dailyProjectRuntimeAgg, &dailyCall)
			addToolCallLocked(dailySessionRuntimeAgg, content.Name, msg.Model)
			addAgentToolCallLocked(dailySessionRuntimeAgg, dailyCall)
			addSessionToolCallLocked(dailySessionRuntimeAgg, dailyCall)
			recordStructuredToolInputLocked(dailySessionRuntimeAgg, &dailyCall)
			if content.Name == "Skill" {
				recordSkillToolUseLocked(dailyRuntimeAgg, dailyCall, dailyCall.SkillName, dailyCall.SkillArgsLength)
				recordSkillToolUseLocked(dailyProjectRuntimeAgg, dailyCall, dailyCall.SkillName, dailyCall.SkillArgsLength)
				recordSkillToolUseLocked(dailySessionRuntimeAgg, dailyCall, dailyCall.SkillName, dailyCall.SkillArgsLength)
			} else if record.SessionID != "" {
				dailyCall.ChainSkills = appendUniqueStrings(dailyCall.ChainSkills, sessionActiveSkills[record.SessionID]...)
				for _, skillName := range sessionActiveSkills[record.SessionID] {
					recordSkillSessionToolLocked(dailyRuntimeAgg, skillName, content.Name, false, false)
					recordSkillSessionToolLocked(dailyProjectRuntimeAgg, skillName, content.Name, false, false)
					recordSkillSessionToolLocked(dailySessionRuntimeAgg, skillName, content.Name, false, false)
				}
			}
			if content.Name != "Skill" && record.AttributionSkill != "" {
				dailyCall.ChainSkills = appendUniqueStrings(dailyCall.ChainSkills, record.AttributionSkill)
				recordSkillSessionToolLocked(dailyRuntimeAgg, record.AttributionSkill, content.Name, false, false)
				recordSkillSessionToolLocked(dailyProjectRuntimeAgg, record.AttributionSkill, content.Name, false, false)
				recordSkillSessionToolLocked(dailySessionRuntimeAgg, record.AttributionSkill, content.Name, false, false)
			}
			pendingTools[content.ID] = call
		}
	}
	if err := scanner.Err(); err != nil {
		Warn("JSONL 文件读取失败", "path", filePath, "error", err.Error())
		return
	}

	if len(pendingTools) > 0 {
		for _, call := range pendingTools {
			addMissingToolResultLocked(agg, call)
			dateKey := call.Timestamp.Format("2006-01-02")
			if dateKey != "0001-01-01" {
				addMissingToolResultLocked(ensureDailyRuntimeAggregate(agg, dateKey), call)
				addMissingToolResultLocked(ensureDailyProjectRuntimeAggregate(agg, dateKey, call.Project), call)
				addMissingToolResultLocked(ensureDailySessionRuntimeAggregate(agg, dateKey, call.SessionID), call)
			}
		}
	}

	// 名字要在所有计数都落完之后补，才能覆盖缺失结果路径新建的 agent 桶。
	applyQoderAgentName(agg, agentName, seenAgentIDs)
}

// requestRoundTripMs 计算本次 assistant 响应与上一条 message 行的时间差(ms)。
// 返回 0 表示无法测量（无前驱、倒序或间隔异常被排除）。间隔 >= 30min 视为会话挂起/中断，不计入。
func requestRoundTripMs(lastMsgTs map[string]time.Time, sessionID string, ts time.Time, hasTs bool) int64 {
	if !hasTs || sessionID == "" {
		return 0
	}
	prev, ok := lastMsgTs[sessionID]
	if !ok {
		return 0
	}
	d := ts.Sub(prev).Milliseconds()
	if d <= 0 || d >= 30*60*1000 {
		return 0
	}
	return d
}

func parseProjectRecordTimestamp(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	timestamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false
	}
	return timestamp, true
}

func hasTimeFilter(tf TimeFilter) bool {
	return tf.Start != nil || tf.End != nil
}

// shouldAccountResponse 判断这条 assistant 记录是否代表一次尚未记账的 API 响应。
//
// 一次响应在 transcript 里被拆成多条记录（thinking/text/tool_use 各一条），Claude 会把
// 同一份 usage 原样重复在每条上（实测 572 个多记录响应中 535 个 usage 完全相同，最多重复
// 8 次）；Qoder 只在其中一条上带 usage。不去重会让请求数虚高 2.3-2.8 倍，token 与成本
// 同步虚高。无 message.id 时无法去重，退回逐条计数以保持旧行为。
func shouldAccountResponse(id string, usage MessageUsage, seen map[string]bool) bool {
	if id == "" {
		return true
	}
	if !usage.hasAccounting() {
		// 不占用记账名额：先遇到的无用量拆分记录不能把真正带 usage 的那条挡在门外。
		return false
	}
	if seen[id] {
		return false
	}
	seen[id] = true
	return true
}

// recordCompactBoundaryLocked 记一次上下文压缩及其代价。
// dropped 用单次 preTokens-postTokens 差值：compactMetadata 里的
// cumulativeDroppedTokens 是全 session 累计量，逐条再求和会重复计数。
func recordCompactBoundaryLocked(agg *ProjectAggregate, record ProjectRecord, dateKey string) {
	var meta struct {
		Trigger            string `json:"trigger"`
		PreTokens          int    `json:"preTokens"`
		PostTokens         int    `json:"postTokens"`
		MessagesSummarized int    `json:"messagesSummarized"`
		DurationMs         int64  `json:"durationMs"`
	}
	// 元数据缺失（旧版本只写 subtype）时仍然计一次压缩。
	_ = json.Unmarshal(record.CompactMetadata, &meta)

	dropped := meta.PreTokens - meta.PostTokens
	if dropped < 0 {
		dropped = 0
	}
	targets := []*ProjectAggregate{agg}
	if dateKey != "" && dateKey != "0001-01-01" {
		targets = append(targets, ensureDailyRuntimeAggregate(agg, dateKey))
	}
	for _, target := range targets {
		stats := ensureContextStats(target)
		stats.ContextCompactions++
		stats.CompactDroppedTokens += dropped
		stats.CompactDurationMs += meta.DurationMs
		stats.CompactMessagesSummary += meta.MessagesSummarized
		if meta.PreTokens > stats.CompactMaxPreTokens {
			stats.CompactMaxPreTokens = meta.PreTokens
		}
	}
}

func ensureContextStats(agg *ProjectAggregate) *ContextAnalysisData {
	if agg.ContextStats == nil {
		agg.ContextStats = &ContextAnalysisData{}
	}
	return agg.ContextStats
}
