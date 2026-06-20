package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
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
	projectsDir := filepath.Join(dataDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("读取 projects 目录失败: %w", err)
	}

	aggregate := newProjectAggregate()

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())
		projectFiles, err := projectJSONLFiles(projectDir)
		if err != nil {
			continue
		}
		files = append(files, projectFiles...)
	}
	sort.Strings(files)

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
				parseProjectFileAggregate(filePath, tf, workerAggregate)
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

	recordInstalledSkillsLocked(aggregate, dataDir)

	// 后处理：生成输出格式数据
	aggregate.finalize()

	return aggregate, nil
}

func projectJSONLFiles(projectDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(projectDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// parseProjectFileAggregate 解析单个项目文件并更新聚合数据
func parseProjectFileAggregate(filePath string, tf TimeFilter, agg *ProjectAggregate) {
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()

	pendingTools := make(map[string]pendingToolCall)
	sessionActiveSkills := make(map[string][]string)
	lastMsgTs := make(map[string]time.Time) // sessionID -> 上一条带 timestamp 的 message 行时间，用于算单请求 round-trip
	decoder := json.NewDecoder(f)
	for {
		var record ProjectRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			continue
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
		if msg.Model != "" {
			tokens := msg.Usage.InputTokens + msg.Usage.OutputTokens
			recordModelUsageLocked(agg, msg.Model, tokens)
			recordModelUsageLocked(dailyRuntimeAgg, msg.Model, tokens)
			recordModelUsageLocked(dailyProjectRuntimeAgg, msg.Model, tokens)
			recordModelUsageLocked(dailySessionRuntimeAgg, msg.Model, tokens)
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
