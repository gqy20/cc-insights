function initToolModelFailureChart(toolAnalysis) {
    const insight = document.getElementById('toolModelFailureChart-insight');
    if (!toolAnalysis || !toolAnalysis.by_model || toolAnalysis.by_model.length === 0) {
        insight.innerHTML = '<strong>数据洞察:</strong> 暂无模型与工具失败关联数据';
        return;
    }

    const chart = echarts.init(document.getElementById('toolModelFailureChart'), 'wonderland');
    const top = toolAnalysis.by_model
        .filter(item => item.call_count > 0)
        .slice(0, 15);

    const labels = top.map(item => `${shortModelName(item.model)} / ${shortToolName(item.tool)}`);
    const failureCounts = top.map(item => item.failure_count);
    const missingCounts = top.map(item => item.missing_result_count);
    const failureRates = top.map(item => Number(item.failure_rate.toFixed(1)));

    chart.setOption({
        title: {
            text: '模型 × 工具失败分析',
            subtext: '全局统计: 按失败数排序',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'shadow' },
            formatter: function(params) {
                const item = top[params[0].dataIndex];
                return `${escapeHtml(item.model)}<br/>${escapeHtml(item.tool)}<br/>` +
                    `调用: ${formatNumber(item.call_count)}<br/>` +
                    `失败: ${formatNumber(item.failure_count)}<br/>` +
                    `Missing: ${formatNumber(item.missing_result_count)}<br/>` +
                    `失败率: ${item.failure_rate.toFixed(1)}%`;
            }
        },
        legend: {
            data: ['失败数', 'Missing Result', '失败率'],
            top: 55
        },
        grid: {
            top: 95,
            bottom: 120,
            left: 70,
            right: 70,
            containLabel: true
        },
        xAxis: {
            type: 'category',
            data: labels,
            axisLabel: {
                interval: 0,
                rotate: 35
            }
        },
        yAxis: [
            { type: 'value', name: '次数' },
            { type: 'value', name: '失败率 %', max: 100 }
        ],
        series: [
            {
                name: '失败数',
                type: 'bar',
                stack: 'failures',
                data: failureCounts,
                itemStyle: { color: '#e74c3c' }
            },
            {
                name: 'Missing Result',
                type: 'bar',
                stack: 'failures',
                data: missingCounts,
                itemStyle: { color: '#f39c12' }
            },
            {
                name: '失败率',
                type: 'line',
                yAxisIndex: 1,
                data: failureRates,
                smooth: true,
                itemStyle: { color: '#2c3e50' }
            }
        ]
    });

    const worst = top[0];
    insight.innerHTML =
        `<strong>数据洞察:</strong> 当前解析到 <strong>${formatNumber(toolAnalysis.total_calls)}</strong> 次工具调用，` +
        `失败 <strong>${formatNumber(toolAnalysis.total_failures)}</strong> 次，` +
        `missing result <strong>${formatNumber(toolAnalysis.missing_results)}</strong> 次。` +
        `失败最多的是 <strong>${escapeHtml(worst.model)} / ${escapeHtml(worst.tool)}</strong>，` +
        `失败率 <strong>${worst.failure_rate.toFixed(1)}%</strong>。`;
}

function initFailureReasonChart(failureAnalysis) {
    const insight = document.getElementById('failureReasonChart-insight');
    if (!failureAnalysis || !failureAnalysis.by_reason || failureAnalysis.by_reason.length === 0) {
        insight.innerHTML = '<strong>数据洞察:</strong> 暂无失败原因细分数据';
        return;
    }

    const chart = echarts.init(document.getElementById('failureReasonChart'), 'wonderland');
    const reasons = failureAnalysis.by_reason.slice(0, 12);
    const toolReasons = (failureAnalysis.by_tool_reason || []).slice(0, 12);
    const reasonLabels = reasons.map(item => `${item.category}/${item.reason}`);
    const toolReasonLabels = toolReasons.map(item => `${shortToolName(item.tool)} / ${item.reason}`);

    chart.setOption({
        title: {
            text: '失败原因细分',
            subtext: '全局统计: 按原因、工具+原因排序',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'shadow' },
            formatter: function(params) {
                const first = params[0];
                if (first.axisIndex === 0) {
                    const item = reasons[first.dataIndex];
                    return `${escapeHtml(item.category)} / ${escapeHtml(item.reason)}<br/>` +
                        `次数: ${formatNumber(item.count)}`;
                }
                const item = toolReasons[first.dataIndex];
                return `${escapeHtml(item.tool)}<br/>${escapeHtml(item.category)} / ${escapeHtml(item.reason)}<br/>` +
                    `次数: ${formatNumber(item.count)}<br/>` +
                    `占工具调用: ${Number(item.rate || 0).toFixed(1)}%`;
            }
        },
        legend: {
            data: ['原因次数', '工具原因次数', '工具原因占比'],
            top: 55
        },
        grid: [
            { top: 95, left: 70, right: 70, height: 180, containLabel: true },
            { top: 370, left: 70, right: 70, height: 170, containLabel: true }
        ],
        xAxis: [
            {
                type: 'category',
                gridIndex: 0,
                data: reasonLabels,
                axisLabel: { interval: 0, rotate: 30 }
            },
            {
                type: 'category',
                gridIndex: 1,
                data: toolReasonLabels,
                axisLabel: { interval: 0, rotate: 30 }
            }
        ],
        yAxis: [
            { type: 'value', gridIndex: 0, name: '失败次数' },
            { type: 'value', gridIndex: 1, name: '工具失败次数' },
            { type: 'value', gridIndex: 1, name: '占比 %', max: 100 }
        ],
        series: [
            {
                name: '原因次数',
                type: 'bar',
                xAxisIndex: 0,
                yAxisIndex: 0,
                data: reasons.map(item => item.count),
                itemStyle: { color: '#e74c3c' }
            },
            {
                name: '工具原因次数',
                type: 'bar',
                xAxisIndex: 1,
                yAxisIndex: 1,
                data: toolReasons.map(item => item.count),
                itemStyle: { color: '#f39c12' }
            },
            {
                name: '工具原因占比',
                type: 'line',
                xAxisIndex: 1,
                yAxisIndex: 2,
                data: toolReasons.map(item => Number(Number(item.rate || 0).toFixed(1))),
                smooth: true,
                itemStyle: { color: '#2c3e50' }
            }
        ]
    });

    const topReason = reasons[0];
    const topToolReason = toolReasons[0];
    insight.innerHTML =
        `<strong>数据洞察:</strong> 共细分 <strong>${formatNumber(failureAnalysis.total_failures)}</strong> 次工具失败。` +
        `最多的原因是 <strong>${escapeHtml(topReason.category)} / ${escapeHtml(topReason.reason)}</strong>，` +
        `出现 <strong>${formatNumber(topReason.count)}</strong> 次。` +
        (topToolReason
            ? `工具维度最突出的是 <strong>${escapeHtml(topToolReason.tool)} / ${escapeHtml(topToolReason.reason)}</strong>。`
            : '');
}

function initEventHookChart(eventAnalysis) {
    const insight = document.getElementById('eventHookChart-insight');
    if (!eventAnalysis || !eventAnalysis.by_type || eventAnalysis.by_type.length === 0) {
        insight.innerHTML = '<strong>数据洞察:</strong> 暂无运行事件数据';
        return;
    }

    const chart = echarts.init(document.getElementById('eventHookChart'), 'wonderland');
    const topEvents = eventAnalysis.by_type.slice(0, 12);
    const hooks = (eventAnalysis.hooks || []).slice(0, 8);
    const eventNames = topEvents.map(item => shortEventName(item.type));
    const hookNames = hooks.map(item => item.hook_event || item.hook_name || 'unknown');

    chart.setOption({
        title: {
            text: '运行事件与 Hook 状态',
            subtext: '全局统计: 事件类型、Hook 成功/取消/错误',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'shadow' }
        },
        legend: {
            data: ['事件数', 'Hook 成功', 'Hook 取消', 'Hook 错误'],
            top: 55
        },
        grid: [
            { top: 95, left: 70, right: 70, height: 170, containLabel: true },
            { top: 345, left: 70, right: 70, height: 130, containLabel: true }
        ],
        xAxis: [
            {
                type: 'category',
                gridIndex: 0,
                data: eventNames,
                axisLabel: { interval: 0, rotate: 30 }
            },
            {
                type: 'category',
                gridIndex: 1,
                data: hookNames,
                axisLabel: { interval: 0, rotate: 25 }
            }
        ],
        yAxis: [
            { type: 'value', gridIndex: 0, name: '事件数' },
            { type: 'value', gridIndex: 1, name: 'Hook 次数' }
        ],
        series: [
            {
                name: '事件数',
                type: 'bar',
                xAxisIndex: 0,
                yAxisIndex: 0,
                data: topEvents.map(item => item.count),
                itemStyle: { color: '#5470c6' }
            },
            {
                name: 'Hook 成功',
                type: 'bar',
                stack: 'hooks',
                xAxisIndex: 1,
                yAxisIndex: 1,
                data: hooks.map(item => item.success_count),
                itemStyle: { color: '#27ae60' }
            },
            {
                name: 'Hook 取消',
                type: 'bar',
                stack: 'hooks',
                xAxisIndex: 1,
                yAxisIndex: 1,
                data: hooks.map(item => item.cancelled_count),
                itemStyle: { color: '#f39c12' }
            },
            {
                name: 'Hook 错误',
                type: 'bar',
                stack: 'hooks',
                xAxisIndex: 1,
                yAxisIndex: 1,
                data: hooks.map(item => item.error_count),
                itemStyle: { color: '#e74c3c' }
            }
        ]
    });

    const topHook = hooks[0];
    const hookText = topHook
        ? `Hook 中 <strong>${escapeHtml(topHook.hook_event || topHook.hook_name)}</strong> 错误/取消最多，失败率 <strong>${topHook.failure_rate.toFixed(1)}%</strong>。`
        : '暂无 Hook 状态记录。';
    insight.innerHTML =
        `<strong>数据洞察:</strong> 共解析 <strong>${formatNumber(eventAnalysis.total_events)}</strong> 个运行事件，` +
        `排队命令 <strong>${formatNumber(eventAnalysis.queued_commands || 0)}</strong> 个，` +
        `计划模式进入 <strong>${formatNumber(eventAnalysis.plan_mode_count || 0)}</strong> 次。` +
        hookText;
}

function initAgentChart(agentAnalysis) {
    const insight = document.getElementById('agentChart-insight');
    if (!agentAnalysis || !agentAnalysis.agents || agentAnalysis.agents.length === 0) {
        insight.innerHTML = '<strong>数据洞察:</strong> 暂无 agent/subagent 数据';
        return;
    }

    const chart = echarts.init(document.getElementById('agentChart'), 'wonderland');
    const agents = agentAnalysis.agents.slice(0, 12);
    const labels = agents.map(item => item.agent_name || shortAgentID(item.agent_id));

    chart.setOption({
        title: {
            text: 'Agent / Subagent 工具调用',
            subtext: '全局统计: 按失败数排序',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'shadow' },
            formatter: function(params) {
                const item = agents[params[0].dataIndex];
                return `${escapeHtml(item.agent_name || item.agent_id)}<br/>` +
                    `类型: ${item.is_sidechain ? 'subagent' : 'main'}<br/>` +
                    `工具调用: ${formatNumber(item.tool_call_count)}<br/>` +
                    `失败: ${formatNumber(item.tool_failure_count)}<br/>` +
                    `Missing: ${formatNumber(item.missing_result_count)}<br/>` +
                    `失败率: ${item.failure_rate.toFixed(1)}%`;
            }
        },
        legend: {
            data: ['工具调用', '失败', 'Missing Result', '失败率'],
            top: 55
        },
        grid: {
            top: 95,
            bottom: 95,
            left: 70,
            right: 70,
            containLabel: true
        },
        xAxis: {
            type: 'category',
            data: labels,
            axisLabel: { interval: 0, rotate: 30 }
        },
        yAxis: [
            { type: 'value', name: '次数' },
            { type: 'value', name: '失败率 %', max: 100 }
        ],
        series: [
            {
                name: '工具调用',
                type: 'bar',
                data: agents.map(item => item.tool_call_count),
                itemStyle: { color: '#5470c6' }
            },
            {
                name: '失败',
                type: 'bar',
                data: agents.map(item => item.tool_failure_count),
                itemStyle: { color: '#e74c3c' }
            },
            {
                name: 'Missing Result',
                type: 'bar',
                data: agents.map(item => item.missing_result_count),
                itemStyle: { color: '#f39c12' }
            },
            {
                name: '失败率',
                type: 'line',
                yAxisIndex: 1,
                data: agents.map(item => Number(item.failure_rate.toFixed(1))),
                itemStyle: { color: '#2c3e50' }
            }
        ]
    });

    const sidechainShare = agentAnalysis.main_tool_calls + agentAnalysis.sidechain_tool_calls > 0
        ? (agentAnalysis.sidechain_tool_calls / (agentAnalysis.main_tool_calls + agentAnalysis.sidechain_tool_calls) * 100).toFixed(1)
        : '0.0';
    insight.innerHTML =
        `<strong>数据洞察:</strong> 主会话工具调用 <strong>${formatNumber(agentAnalysis.main_tool_calls)}</strong> 次，` +
        `subagent 工具调用 <strong>${formatNumber(agentAnalysis.sidechain_tool_calls)}</strong> 次，占比 <strong>${sidechainShare}%</strong>。` +
        `图中按失败数展示最需要关注的 agent。`;
}

function initCommandFileChart(commandAnalysis) {
    const insight = document.getElementById('commandFileChart-insight');
    if (!commandAnalysis) {
        insight.innerHTML = '<strong>数据洞察:</strong> 暂无 Bash 与文件操作数据';
        return;
    }

    const chart = echarts.init(document.getElementById('commandFileChart'), 'wonderland');
    const bash = (commandAnalysis.bash_commands || []).slice(0, 12);
    const files = (commandAnalysis.file_operations || []).slice(0, 12);
    const bashLabels = bash.map(item => item.command_name);
    const fileLabels = files.map(item => `${item.operation} ${shortPath(item.path)}`);

    chart.setOption({
        title: {
            text: 'Bash 与文件操作失败',
            subtext: '全局统计: Bash Top 与文件失败 Top',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'shadow' }
        },
        legend: {
            data: ['Bash 成功', 'Bash 失败', '文件成功', '文件失败', '文件 Missing'],
            top: 55
        },
        grid: [
            { top: 95, left: 70, right: 70, height: 170, containLabel: true },
            { top: 360, left: 70, right: 70, height: 170, containLabel: true }
        ],
        xAxis: [
            {
                type: 'category',
                gridIndex: 0,
                data: bashLabels,
                axisLabel: { interval: 0, rotate: 30 }
            },
            {
                type: 'category',
                gridIndex: 1,
                data: fileLabels,
                axisLabel: { interval: 0, rotate: 30 }
            }
        ],
        yAxis: [
            { type: 'value', gridIndex: 0, name: 'Bash 次数' },
            { type: 'value', gridIndex: 1, name: '文件次数' }
        ],
        series: [
            {
                name: 'Bash 成功',
                type: 'bar',
                stack: 'bash',
                xAxisIndex: 0,
                yAxisIndex: 0,
                data: bash.map(item => item.success_count),
                itemStyle: { color: '#27ae60' }
            },
            {
                name: 'Bash 失败',
                type: 'bar',
                stack: 'bash',
                xAxisIndex: 0,
                yAxisIndex: 0,
                data: bash.map(item => item.failure_count),
                itemStyle: { color: '#e74c3c' }
            },
            {
                name: '文件成功',
                type: 'bar',
                stack: 'files',
                xAxisIndex: 1,
                yAxisIndex: 1,
                data: files.map(item => item.success_count),
                itemStyle: { color: '#27ae60' }
            },
            {
                name: '文件失败',
                type: 'bar',
                stack: 'files',
                xAxisIndex: 1,
                yAxisIndex: 1,
                data: files.map(item => item.failure_count),
                itemStyle: { color: '#e74c3c' }
            },
            {
                name: '文件 Missing',
                type: 'bar',
                stack: 'files',
                xAxisIndex: 1,
                yAxisIndex: 1,
                data: files.map(item => item.missing_result_count),
                itemStyle: { color: '#f39c12' }
            }
        ]
    });

    const topBash = bash[0];
    const topFile = files[0];
    const riskyCount = (commandAnalysis.risky_commands || []).length;
    insight.innerHTML =
        `<strong>数据洞察:</strong> Bash 命令 Top 中最常见的是 <strong>${escapeHtml(topBash ? topBash.command_name : '-')}</strong>，` +
        `高风险命令类型 <strong>${formatNumber(riskyCount)}</strong> 个。` +
        `文件操作失败最多的是 <strong>${escapeHtml(topFile ? `${topFile.operation} ${topFile.path}` : '-')}</strong>。`;
}

function initCostAnalysisChart(costAnalysis) {
    const insight = document.getElementById('costAnalysisChart-insight');
    if (!costAnalysis || !costAnalysis.totals || !costAnalysis.by_model || costAnalysis.by_model.length === 0) {
        insight.innerHTML = '<strong>数据洞察:</strong> 暂无 Token 与成本数据';
        return;
    }

    const chart = echarts.init(document.getElementById('costAnalysisChart'), 'wonderland');
    const models = (costAnalysis.by_model || []).slice(0, 10);
    const projects = (costAnalysis.by_project || []).slice(0, 10);
    const sessions = (costAnalysis.by_session || []).slice(0, 10);
    const modelLabels = models.map(item => shortModelName(item.model));
    const projectLabels = projects.map(item => shortPath(item.project));
    const sessionLabels = sessions.map(item => shortAgentID(item.session_id));

    chart.setOption({
        title: {
            text: 'Token 与成本分析',
            subtext: '按模型、项目、会话统计 Token 消耗',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'shadow' }
        },
        legend: {
            data: ['输入', '输出', '缓存读取', '缓存写入', '总 Token'],
            top: 55
        },
        grid: [
            { top: 95, left: 70, right: 70, height: 130, containLabel: true },
            { top: 300, left: 70, right: 70, height: 110, containLabel: true },
            { top: 485, left: 70, right: 70, height: 90, containLabel: true }
        ],
        xAxis: [
            { type: 'category', gridIndex: 0, data: modelLabels, axisLabel: { interval: 0, rotate: 25 } },
            { type: 'category', gridIndex: 1, data: projectLabels, axisLabel: { interval: 0, rotate: 25 } },
            { type: 'category', gridIndex: 2, data: sessionLabels, axisLabel: { interval: 0, rotate: 25 } }
        ],
        yAxis: [
            { type: 'value', gridIndex: 0, name: '模型 Token' },
            { type: 'value', gridIndex: 1, name: '项目 Token' },
            { type: 'value', gridIndex: 2, name: '会话 Token' }
        ],
        series: [
            {
                name: '输入',
                type: 'bar',
                stack: 'model',
                xAxisIndex: 0,
                yAxisIndex: 0,
                data: models.map(item => item.input_tokens),
                itemStyle: { color: '#5470c6' }
            },
            {
                name: '输出',
                type: 'bar',
                stack: 'model',
                xAxisIndex: 0,
                yAxisIndex: 0,
                data: models.map(item => item.output_tokens),
                itemStyle: { color: '#91cc75' }
            },
            {
                name: '缓存读取',
                type: 'bar',
                stack: 'model',
                xAxisIndex: 0,
                yAxisIndex: 0,
                data: models.map(item => item.cache_read_input_tokens),
                itemStyle: { color: '#73c0de' }
            },
            {
                name: '缓存写入',
                type: 'bar',
                stack: 'model',
                xAxisIndex: 0,
                yAxisIndex: 0,
                data: models.map(item => item.cache_creation_input_tokens),
                itemStyle: { color: '#fac858' }
            },
            {
                name: '总 Token',
                type: 'bar',
                xAxisIndex: 1,
                yAxisIndex: 1,
                data: projects.map(item => item.total_tokens),
                itemStyle: { color: '#3ba272' }
            },
            {
                name: '总 Token',
                type: 'bar',
                xAxisIndex: 2,
                yAxisIndex: 2,
                data: sessions.map(item => item.total_tokens),
                itemStyle: { color: '#fc8452' }
            }
        ]
    });

    const totals = costAnalysis.totals;
    const topModel = models[0];
    const topProject = projects[0];
    const topSession = sessions[0];
    const cacheInputTotal = (totals.input_tokens || 0) + (totals.cache_read_input_tokens || 0) + (totals.cache_creation_input_tokens || 0);
    const cacheReadRatio = cacheInputTotal > 0
        ? ((totals.cache_read_input_tokens || 0) / cacheInputTotal * 100).toFixed(1)
        : '0.0';

    insight.innerHTML =
        `<strong>数据洞察:</strong> 共 <strong>${formatNumber(totals.request_count)}</strong> 次模型请求，` +
        `总 Token <strong>${formatNumber(totals.total_tokens)}</strong>，输出 Token <strong>${formatNumber(totals.output_tokens)}</strong>。` +
        `缓存读取占输入侧 <strong>${cacheReadRatio}%</strong>。` +
        `最高模型 <strong>${escapeHtml(topModel ? topModel.model : '-')}</strong>，` +
        `最高项目 <strong>${escapeHtml(topProject ? topProject.project : '-')}</strong>，` +
        `最高会话 <strong>${escapeHtml(topSession ? topSession.session_id : '-')}</strong>。`;
}

function initSessionAnalysisChart(sessionAnalysis) {
    const insight = document.getElementById('sessionAnalysisChart-insight');
    if (!sessionAnalysis || !sessionAnalysis.sessions || sessionAnalysis.sessions.length === 0) {
        insight.innerHTML = '<strong>数据洞察:</strong> 暂无 Session 生命周期数据';
        return;
    }

    const chart = echarts.init(document.getElementById('sessionAnalysisChart'), 'wonderland');
    const failures = (sessionAnalysis.top_failures || []).slice(0, 10);
    const longRunning = (sessionAnalysis.long_running || []).slice(0, 10);
    const outcomes = sessionAnalysis.outcomes || [];
    const failureLabels = failures.map(item => shortSessionTitle(item));
    const durationLabels = longRunning.map(item => shortSessionTitle(item));
    const outcomeLabels = outcomes.map(item => item.outcome);

    chart.setOption({
        title: {
            text: 'Session 生命周期复盘',
            subtext: '高失败、高耗时与结果状态',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'shadow' },
            formatter: function(params) {
                const first = params[0];
                if (first.axisIndex === 0) {
                    const item = failures[first.dataIndex];
                    return sessionTooltip(item);
                }
                if (first.axisIndex === 1) {
                    const item = longRunning[first.dataIndex];
                    return sessionTooltip(item);
                }
                const item = outcomes[first.dataIndex];
                return `${escapeHtml(item.outcome)}<br/>数量: ${formatNumber(item.count)}`;
            }
        },
        legend: {
            data: ['工具失败', 'Missing', '耗时分钟', 'Outcome'],
            top: 55
        },
        grid: [
            { top: 95, left: 70, right: 70, height: 145, containLabel: true },
            { top: 315, left: 70, right: 70, height: 120, containLabel: true },
            { top: 500, left: 70, right: 70, height: 75, containLabel: true }
        ],
        xAxis: [
            { type: 'category', gridIndex: 0, data: failureLabels, axisLabel: { interval: 0, rotate: 25 } },
            { type: 'category', gridIndex: 1, data: durationLabels, axisLabel: { interval: 0, rotate: 25 } },
            { type: 'category', gridIndex: 2, data: outcomeLabels, axisLabel: { interval: 0 } }
        ],
        yAxis: [
            { type: 'value', gridIndex: 0, name: '失败次数' },
            { type: 'value', gridIndex: 1, name: '分钟' },
            { type: 'value', gridIndex: 2, name: 'Session' }
        ],
        series: [
            {
                name: '工具失败',
                type: 'bar',
                stack: 'fail',
                xAxisIndex: 0,
                yAxisIndex: 0,
                data: failures.map(item => item.tool_failure_count),
                itemStyle: { color: '#e74c3c' }
            },
            {
                name: 'Missing',
                type: 'bar',
                stack: 'fail',
                xAxisIndex: 0,
                yAxisIndex: 0,
                data: failures.map(item => item.missing_result_count),
                itemStyle: { color: '#f39c12' }
            },
            {
                name: '耗时分钟',
                type: 'bar',
                xAxisIndex: 1,
                yAxisIndex: 1,
                data: longRunning.map(item => Number(((item.duration_ms || 0) / 60000).toFixed(1))),
                itemStyle: { color: '#5470c6' }
            },
            {
                name: 'Outcome',
                type: 'bar',
                xAxisIndex: 2,
                yAxisIndex: 2,
                data: outcomes.map(item => item.count),
                itemStyle: { color: '#3ba272' }
            }
        ]
    });

    const worst = failures[0];
    const longest = longRunning[0];
    insight.innerHTML =
        `<strong>数据洞察:</strong> 当前输出 <strong>${formatNumber(sessionAnalysis.sessions.length)}</strong> 个 Session 摘要。` +
        `失败最多的是 <strong>${escapeHtml(worst ? (worst.title || worst.session_id) : '-')}</strong>，` +
        `工具失败 <strong>${formatNumber(worst ? worst.tool_failure_count : 0)}</strong> 次。` +
        `耗时最长的是 <strong>${escapeHtml(longest ? (longest.title || longest.session_id) : '-')}</strong>，` +
        `约 <strong>${formatNumber(longest ? Math.round((longest.duration_ms || 0) / 60000) : 0)}</strong> 分钟。`;
}

function shortSessionTitle(item) {
    if (!item) return 'unknown';
    const value = item.title || item.session_id || 'unknown';
    return value.length > 28 ? value.slice(0, 28) + '...' : value;
}

function sessionTooltip(item) {
    if (!item) return '';
    return `${escapeHtml(item.title || item.session_id)}<br/>` +
        `项目: ${escapeHtml(shortPath(item.project))}<br/>` +
        `结果: ${escapeHtml(item.outcome || 'unknown')}<br/>` +
        `工具调用: ${formatNumber(item.tool_call_count)}<br/>` +
        `失败: ${formatNumber(item.tool_failure_count)}<br/>` +
        `Missing: ${formatNumber(item.missing_result_count)}<br/>` +
        `Token: ${formatNumber(item.total_tokens)}<br/>` +
        `耗时: ${formatNumber(Math.round((item.duration_ms || 0) / 60000))} 分钟`;
}
