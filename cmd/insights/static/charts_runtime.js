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

// --- file_analysis: 文件与编辑质量分析 ---

function initFileAnalysisChart(fileAnalysis) {
    const insight = document.getElementById('fileAnalysisChart-insight');
    if (!fileAnalysis || !fileAnalysis.totals) {
        insight.innerHTML = '<strong>数据洞察:</strong> 暂无文件编辑分析数据';
        return;
    }

    const chart = echarts.init(document.getElementById('fileAnalysisChart'), 'wonderland');
    const hotFiles = (fileAnalysis.hot_files || []).slice(0, 15);
    const editFailures = (fileAnalysis.edit_failures || []).slice(0, 10);
    const snapshots = (fileAnalysis.snapshots || []).slice(0, 10);

    const hotLabels = hotFiles.map(item => shortPath(item.path));
    const failLabels = editFailures.map(item => shortPath(item.path));
    const snapLabels = snapshots.map(item => shortPath(item.path));

    // 为 Grid 2 准备堆叠数据：按失败原因分系列
    const allReasons = new Set();
    editFailures.forEach(item => (item.failure_reasons || []).forEach(r => allReasons.add(r.reason)));
    const reasonList = Array.from(allReasons).slice(0, 6);
    const reasonColors = ['#e74c3c', '#e67e22', '#9b59b6', '#3498db', '#1abc9c', '#95a5a6'];

    chart.setOption({
        title: { text: '文件与编辑质量分析', subtext: '热门文件 · 编辑失败 · 快照热点', left: 'center' },
        tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
        legend: {
            data: ['Read', 'Edit', 'Write', '失败率%', ...reasonList],
            top: 50,
            textStyle: { fontSize: 10 }
        },
        grid: [
            { top: 90, left: 70, right: 70, height: 180, containLabel: true },
            { top: 300, left: 70, right: 70, height: 170, containLabel: true },
            { top: 500, left: 70, right: 350, height: 150, containLabel: true }
        ],
        xAxis: [
            { type: 'category', gridIndex: 0, data: hotLabels, axisLabel: { interval: 0, rotate: 25, fontSize: 10 } },
            { type: 'category', gridIndex: 1, data: failLabels, axisLabel: { interval: 0, rotate: 25, fontSize: 10 } },
            { type: 'category', gridIndex: 2, data: snapLabels, axisLabel: { interval: 0, rotate: 20, fontSize: 9 } }
        ],
        yAxis: [
            { type: 'value', gridIndex: 0, name: '操作次数' },
            { type: 'value', gridIndex: 1, name: '失败次数' },
            { type: 'value', gridIndex: 2, name: '跨会话数' }
        ],
        series: [
            // Grid 1: 热门文件 - 堆叠柱状
            { name: 'Read', type: 'bar', stack: 'ops', xAxisIndex: 0, yAxisIndex: 0, data: hotFiles.map(i => i.read_count), itemStyle: { color: '#3498db' } },
            { name: 'Edit', type: 'bar', stack: 'ops', xAxisIndex: 0, yAxisIndex: 0, data: hotFiles.map(i => i.edit_count), itemStyle: { color: '#27ae60' } },
            { name: 'Write', type: 'bar', stack: 'ops', xAxisIndex: 0, yAxisIndex: 0, data: hotFiles.map(i => i.write_count), itemStyle: { color: '#f39c12' } },
            { name: '失败率%', type: 'line', xAxisIndex: 0, yAxisIndex: 0, data: hotFiles.map(i => +i.failure_rate.toFixed(1)), itemStyle: { color: '#c0392b' }, lineStyle: { width: 2 }, symbol: 'circle', symbolSize: 6 },
            // Grid 2: 编辑失败 - 按原因堆叠
            ...reasonList.map((reason, idx) => ({
                name: reason, type: 'bar', stack: 'fail', xAxisIndex: 1, yAxisIndex: 1,
                data: editFailures.map(item => {
                    const found = (item.failure_reasons || []).find(r => r.reason === reason);
                    return found ? found.count : 0;
                }),
                itemStyle: { color: reasonColors[idx % reasonColors.length] }
            })),
            // Grid 3: 快照热度
            { name: '跨会话数', type: 'bar', xAxisIndex: 2, yAxisIndex: 2, data: snapshots.map(i => i.session_count), itemStyle: { color: '#8e44ad' }, barMaxWidth: 30 }
        ]
    });

    // Insight 文字
    const t = fileAnalysis.totals;
    const topHot = hotFiles[0];
    const topFail = editFailures[0];
    const topSnap = snapshots[0];
    const topFailReasons = topFail ? (topFail.failure_reasons || []) : [];
    const primaryReason = topFailReasons[0] ? topFailReasons[0].reason : '-';

    insight.innerHTML =
        '<strong>数据洞察:</strong> 共追踪 <strong>' + formatNumber(t.unique_files) + '</strong> 个唯一文件，' +
        'Read <strong>' + formatNumber(t.total_reads) + '</strong> 次 / Edit <strong>' + formatNumber(t.total_edits) + '</strong> 次 / Write <strong>' + formatNumber(t.total_writes) + '</strong> 次。' +
        'Edit 整体失败率 <strong>' + t.overall_edit_failure_rate.toFixed(1) + '%</strong>。' +
        '最活跃文件是 <strong>' + escapeHtml(topHot ? topHot.path : '-') + '</strong> (' + (topHot ? topHot.total_ops : 0) + ' 次操作)。' +
        (topFail ? '最易失败的文件是 <strong>' + escapeHtml(topFail.path) + '</strong> (' + topFail.total_failures + ' 次失败，主要因 <strong>' + primaryReason + '</strong>)。' : '') +
        (topSnap ? '快照热点文件 <strong>' + escapeHtml(topSnap.path) + '</strong> 出现在 <strong>' + topSnap.session_count + '</strong> 个会话中，最高版本 v' + topSnap.max_version + '。' : '');
}

// --- task_plan_analysis (M4): Task / Plan 结构分析 ---

function initTaskPlanChart(tpa) {
    const insight = document.getElementById('taskPlanChart-insight');
    if (!tpa) {
        insight.innerHTML = '<strong>数据洞察:</strong> 暂无 Task / Plan 分析数据';
        return;
    }

    const chart = echarts.init(document.getElementById('taskPlanChart'), 'wonderland');
    const lc = tpa.plan_lifecycle || {};
    const planFiles = (tpa.plan_files || []).slice(0, 10);
    const tasks = tpa.tasks || {};
    const reminder = tpa.reminder_summary || {};

    // Grid 1: Plan Lifecycle (左上)
    const lifecycleLabels = ['进入', '退出', '重入'];
    const lifecycleData = [lc.entry_count || 0, lc.exit_count || 0, lc.reentry_count || 0];

    // Grid 2: Plan Files Top (右上)
    const fileLabels = planFiles.map(f => shortPath(f.file_path || f.file_name));
    const fileRefCounts = planFiles.map(f => f.ref_count);

    // Grid 3: Task Status Distribution (左下)
    const statusDist = tasks.status_distribution || [];
    const statusLabels = statusDist.map(s => s.status);
    const completedData = statusDist.find(s => s.status === 'completed') || { count: 0 };
    const pendingData = statusDist.find(s => s.status === 'pending') || { count: 0 };
    const inProgressData = statusDist.find(s => s.status === 'in_progress') || { count: 0 };

    // Grid 4: Reminder Frequency (右下)
    const taskSessions = (reminder.top_task_sessions || []).slice(0, 8);
    const todoSessions = (reminder.top_todo_sessions || []).slice(0, 8);
    const reminderLabels = [...new Set([
        ...taskSessions.map(s => shortAgentID(s.session_id)),
        ...todoSessions.map(s => shortAgentID(s.session_id))
    ])].slice(0, 8);

    // Grid 5: Per-session Task Counts (底部全宽)
    const sessionTasks = (tasks.session_task_counts || []).slice(0, 10);
    const sessionTaskLabels = sessionTasks.map(s => shortAgentID(s.session_id));

    chart.setOption({
        title: {
            text: 'Task / Plan 结构分析',
            subtext: 'Plan Mode 生命周期 · 计划文件引用 · 任务状态分布 · 提醒频率',
            left: 'center'
        },
        tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
        legend: {
            data: ['进入', '退出', '重入', '引用次数', 'Completed', 'Pending', 'InProgress', 'Task Reminder', 'Todo Reminder'],
            top: 50,
            textStyle: { fontSize: 10 }
        },
        grid: [
            { top: 95, left: 55, right: 55, height: 140, containLabel: true },
            { top: 95, left: 420, right: 30, height: 140, containLabel: true },
            { top: 280, left: 55, right: 55, height: 160, containLabel: true },
            { top: 280, left: 420, right: 30, height: 160, containLabel: true },
            { top: 480, left: 55, right: 30, height: 180, containLabel: true }
        ],
        xAxis: [
            { type: 'category', gridIndex: 0, data: lifecycleLabels },
            { type: 'value', gridIndex: 1, name: '引用次数' },
            { type: 'category', gridIndex: 2, data: statusLabels.length > 0 ? statusLabels : ['completed', 'pending', 'in_progress'] },
            { type: 'category', gridIndex: 3, data: reminderLabels.length > 0 ? reminderLabels : ['-'] },
            { type: 'value', gridIndex: 4, name: '任务数' }
        ],
        yAxis: [
            { type: 'value', gridIndex: 0, name: '次数' },
            { type: 'category', gridIndex: 1, data: fileLabels, inverse: true, axisLabel: { fontSize: 9 } },
            { type: 'value', gridIndex: 2, name: '任务数' },
            { type: 'value', gridIndex: 3, name: '次数' },
            { type: 'category', gridIndex: 4, data: sessionTaskLabels, inverse: true, axisLabel: { fontSize: 9 } }
        ],
        series: [
            { name: '进入', type: 'bar', xAxisIndex: 0, yAxisIndex: 0, data: [lifecycleData[0], 0, 0], itemStyle: { color: '#27ae60' }, barMaxWidth: 40 },
            { name: '退出', type: 'bar', xAxisIndex: 0, yAxisIndex: 0, data: [0, lifecycleData[1], 0], itemStyle: { color: '#e74c3c' }, barMaxWidth: 40 },
            { name: '重入', type: 'bar', xAxisIndex: 0, yAxisIndex: 0, data: [0, 0, lifecycleData[2]], itemStyle: { color: '#f39c12' }, barMaxWidth: 40 },
            { name: '引用次数', type: 'bar', xAxisIndex: 1, yAxisIndex: 1, data: fileRefCounts, itemStyle: { color: '#8e44ad' }, barMaxWidth: 20 },
            { name: 'Completed', type: 'bar', stack: 'status', xAxisIndex: 2, yAxisIndex: 2,
              data: statusLabels.map(l => l === 'completed' ? (completedData.count || 0) : 0), itemStyle: { color: '#27ae60' } },
            { name: 'Pending', type: 'bar', stack: 'status', xAxisIndex: 2, yAxisIndex: 2,
              data: statusLabels.map(l => l === 'pending' ? (pendingData.count || 0) : 0), itemStyle: { color: '#f39c12' } },
            { name: 'InProgress', type: 'bar', stack: 'status', xAxisIndex: 2, yAxisIndex: 2,
              data: statusLabels.map(l => l === 'in_progress' ? (inProgressData.count || 0) : 0), itemStyle: { color: '#3498db' } },
            { name: 'Task Reminder', type: 'bar', xAxisIndex: 3, yAxisIndex: 3,
              data: reminderLabels.map(sid => {
                  const found = taskSessions.find(s => s.session_id === sid);
                  return found ? found.count : 0;
              }), itemStyle: { color: '#e67e22' } },
            { name: 'Todo Reminder', type: 'bar', xAxisIndex: 3, yAxisIndex: 3,
              data: reminderLabels.map(sid => {
                  const found = todoSessions.find(s => s.session_id === sid);
                  return found ? found.count : 0;
              }), itemStyle: { color: '#16a085' } },
            { name: 'Completed', type: 'bar', stack: 'sessionTasks', xAxisIndex: 4, yAxisIndex: 4,
              data: sessionTasks.map(s => s.completed_count), itemStyle: { color: '#27ae60' }, barMaxWidth: 18 },
            { name: 'Pending', type: 'bar', stack: 'sessionTasks', xAxisIndex: 4, yAxisIndex: 4,
              data: sessionTasks.map(s => s.pending_count), itemStyle: { color: '#f39c12' }, barMaxWidth: 18 },
            { name: 'InProgress', type: 'bar', stack: 'sessionTasks', xAxisIndex: 4, yAxisIndex: 4,
              data: sessionTasks.map(s => s.in_progress_count), itemStyle: { color: '#3498db' }, barMaxWidth: 18 }
        ]
    });

    const exitRate = lc.entry_count > 0 ? ((lc.exit_count / lc.entry_count) * 100).toFixed(0) : '0';
    const topFile = planFiles[0];
    const goalCount = (tpa.goal_status || []).length;

    insight.innerHTML =
        '<strong>数据洞察:</strong> Plan mode 进入 <strong>' + formatNumber(lc.entry_count) + '</strong> 次，' +
        '退出 <strong>' + formatNumber(lc.exit_count) + '</strong> 次（完成率 <strong>' + exitRate + '%</strong>），' +
        '涉及 <strong>' + formatNumber(lc.unique_plans) + '</strong> 个唯一计划文件。' +
        (topFile ? '最常引用的计划是 <strong>' + escapeHtml(shortPath(topFile.file_path || topFile.file_name)) + '</strong>（' + topFile.ref_count + ' 次）。' : '') +
        '任务总数 <strong>' + formatNumber(tasks.total_tasks || 0) + '</strong> 个，' +
        '完成率 <strong>' + (tasks.completion_rate || 0).toFixed(1) + '%</strong>，' +
        '平均每 Session <strong>' + (tasks.avg_tasks_per_session || 0).toFixed(1) + '</strong> 个任务。' +
        '提醒总次数 <strong>' + formatNumber((reminder.task_reminder_count || 0) + (reminder.todo_reminder_count || 0)) + '</strong> 次。' +
        (goalCount > 0 ? '目标达成事件 <strong>' + goalCount + '</strong> 条。' : '');
}
