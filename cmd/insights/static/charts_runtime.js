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
    const reasons = failureAnalysis.by_reason.slice(0, 8);
    const toolReasons = (failureAnalysis.by_tool_reason || []).slice(0, 8);
    const reasonLabels = reasons.map(item => `${item.category}/${item.reason}`);
    const toolReasonLabels = toolReasons.map(item => `${shortToolName(item.tool)} / ${item.reason}`);

    chart.setOption({
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'shadow',
                shadowStyle: { color: 'rgba(47, 128, 209, 0.08)' }
            },
            formatter: function(params) {
                const first = params[0];
                if (first.seriesIndex === 0) {
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
            data: ['原因次数', '工具原因次数'],
            top: 0,
            left: 0,
            itemWidth: 10,
            itemHeight: 10,
            textStyle: { color: '#596579' }
        },
        grid: [
            { top: 48, left: 150, right: 52, height: 190, containLabel: false },
            { top: 322, left: 150, right: 52, height: 210, containLabel: false }
        ],
        xAxis: [
            {
                type: 'value',
                gridIndex: 0,
                splitNumber: 4,
                axisLine: { show: false },
                axisTick: { show: false },
                axisLabel: { color: '#8a95a3' },
                splitLine: { lineStyle: { color: '#edf2f7', type: 'dashed' } }
            },
            {
                type: 'value',
                gridIndex: 1,
                splitNumber: 4,
                axisLine: { show: false },
                axisTick: { show: false },
                axisLabel: { color: '#8a95a3' },
                splitLine: { lineStyle: { color: '#edf2f7', type: 'dashed' } }
            }
        ],
        yAxis: [
            {
                type: 'category',
                gridIndex: 0,
                data: reasonLabels,
                inverse: true,
                axisLine: { show: false },
                axisTick: { show: false },
                axisLabel: {
                    color: '#4b5563',
                    width: 138,
                    overflow: 'truncate',
                    interval: 0
                }
            },
            {
                type: 'category',
                gridIndex: 1,
                data: toolReasonLabels,
                inverse: true,
                axisLine: { show: false },
                axisTick: { show: false },
                axisLabel: {
                    color: '#4b5563',
                    width: 138,
                    overflow: 'truncate',
                    interval: 0
                }
            }
        ],
        series: [
            {
                name: '原因次数',
                type: 'bar',
                xAxisIndex: 0,
                yAxisIndex: 0,
                data: reasons.map(item => item.count),
                barMaxWidth: 18,
                label: {
                    show: true,
                    position: 'right',
                    color: '#596579',
                    fontSize: 11,
                    formatter: value => formatNumber(value.value)
                },
                itemStyle: {
                    color: '#c93838',
                    borderRadius: [0, 4, 4, 0]
                }
            },
            {
                name: '工具原因次数',
                type: 'bar',
                xAxisIndex: 1,
                yAxisIndex: 1,
                data: toolReasons.map(item => item.count),
                barMaxWidth: 18,
                label: {
                    show: true,
                    position: 'right',
                    color: '#596579',
                    fontSize: 11,
                    formatter: value => formatNumber(value.value)
                },
                itemStyle: {
                    color: '#d97706',
                    borderRadius: [0, 4, 4, 0]
                }
            }
        ],
        graphic: [
            {
                type: 'text',
                left: 0,
                top: 28,
                style: {
                    text: '全局原因 Top 8',
                    fill: '#1f2933',
                    fontSize: 12,
                    fontWeight: 650
                }
            },
            {
                type: 'text',
                left: 0,
                top: 300,
                style: {
                    text: '工具 + 原因 Top 8',
                    fill: '#1f2933',
                    fontSize: 12,
                    fontWeight: 650
                }
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
            axisPointer: { type: 'shadow' },
            formatter: function(params) {
                return params.map(item =>
                    `${escapeHtml(item.seriesName)}: ${formatTokenCount(item.value)}`
                ).join('<br/>');
            }
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
            { type: 'value', gridIndex: 0, name: '模型 Token', axisLabel: { formatter: value => formatTokenCount(value) } },
            { type: 'value', gridIndex: 1, name: '项目 Token', axisLabel: { formatter: value => formatTokenCount(value) } },
            { type: 'value', gridIndex: 2, name: '会话 Token', axisLabel: { formatter: value => formatTokenCount(value) } }
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
        `总 Token <strong>${formatTokenCount(totals.total_tokens)}</strong>，输出 Token <strong>${formatTokenCount(totals.output_tokens)}</strong>。` +
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
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'shadow',
                shadowStyle: { color: 'rgba(47, 128, 209, 0.08)' }
            },
            formatter: function(params) {
                const first = params[0];
                if (first.axisIndex === 0 || first.seriesIndex <= 1) {
                    const item = failures[first.dataIndex];
                    return sessionTooltip(item);
                }
                if (first.axisIndex === 1 || first.seriesIndex === 2) {
                    const item = longRunning[first.dataIndex];
                    return sessionTooltip(item);
                }
                const item = outcomes[first.dataIndex];
                return `${escapeHtml(item.outcome)}<br/>数量: ${formatNumber(item.count)}`;
            }
        },
        legend: {
            data: ['工具失败', 'Missing', '耗时分钟', 'Outcome'],
            top: 0,
            left: 0,
            itemWidth: 10,
            itemHeight: 10,
            textStyle: { color: '#596579' }
        },
        grid: [
            { top: 48, left: 170, right: 58, height: 160, containLabel: false },
            { top: 282, left: 170, right: 58, height: 145, containLabel: false },
            { top: 500, left: 110, right: 58, height: 86, containLabel: false }
        ],
        xAxis: [
            makeTaskPlanValueAxis(0),
            makeTaskPlanValueAxis(1),
            makeTaskPlanValueAxis(2)
        ],
        yAxis: [
            makeTaskPlanCategoryAxis(0, failureLabels, 150),
            makeTaskPlanCategoryAxis(1, durationLabels, 150),
            makeTaskPlanCategoryAxis(2, outcomeLabels, 92)
        ],
        series: [
            {
                name: '工具失败',
                type: 'bar',
                stack: 'fail',
                xAxisIndex: 0,
                yAxisIndex: 0,
                data: failures.map(item => item.tool_failure_count),
                barMaxWidth: 18,
                itemStyle: { color: '#c93838' }
            },
            {
                name: 'Missing',
                type: 'bar',
                stack: 'fail',
                xAxisIndex: 0,
                yAxisIndex: 0,
                data: failures.map(item => item.missing_result_count),
                barMaxWidth: 18,
                itemStyle: { color: '#d97706' }
            },
            {
                name: '耗时分钟',
                type: 'bar',
                xAxisIndex: 1,
                yAxisIndex: 1,
                data: longRunning.map(item => Number(((item.duration_ms || 0) / 60000).toFixed(1))),
                barMaxWidth: 18,
                label: {
                    show: true,
                    position: 'right',
                    color: '#596579',
                    fontSize: 11,
                    formatter: value => formatNumber(Math.round(value.value))
                },
                itemStyle: {
                    color: '#4f6ec8',
                    borderRadius: [0, 4, 4, 0]
                }
            },
            {
                name: 'Outcome',
                type: 'bar',
                xAxisIndex: 2,
                yAxisIndex: 2,
                data: outcomes.map(item => item.count),
                barMaxWidth: 18,
                label: {
                    show: true,
                    position: 'right',
                    color: '#596579',
                    fontSize: 11,
                    formatter: value => formatNumber(value.value)
                },
                itemStyle: {
                    color: '#3ba272',
                    borderRadius: [0, 4, 4, 0]
                }
            }
        ],
        graphic: [
            makeChartSectionLabel('失败最多的 Session', 0, 24),
            makeChartSectionLabel('耗时最长的 Session', 0, 258),
            makeChartSectionLabel('Outcome 分布', 0, 476)
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
        `Token: ${formatTokenCount(item.total_tokens)}<br/>` +
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

    // Grid 1: Plan lifecycle and task status
    const overviewRows = [
        { label: 'Plan 进入', value: lc.entry_count || 0, color: '#21875a' },
        { label: 'Plan 退出', value: lc.exit_count || 0, color: '#c93838' },
        { label: 'Plan 重入', value: lc.reentry_count || 0, color: '#d97706' }
    ];

    const statusDist = tasks.status_distribution || [];
    const statusLabels = {
        completed: '任务完成',
        pending: '任务待办',
        in_progress: '任务进行中'
    };
    statusDist
        .filter(item => Number(item.count || 0) > 0)
        .forEach(item => {
            overviewRows.push({
                label: statusLabels[item.status] || item.status,
                value: item.count || 0,
                color: item.status === 'completed' ? '#21875a' : item.status === 'pending' ? '#d97706' : '#2f80d1'
            });
        });

    // Grid 2: Plan Files Top
    const fileRows = planFiles
        .filter(f => Number(f.ref_count || 0) > 0)
        .map(f => ({
            label: shortPath(f.file_path || f.file_name),
            value: f.ref_count || 0
        }));

    // Grid 3: Reminder Frequency
    const taskSessions = (reminder.top_task_sessions || []).slice(0, 8);
    const todoSessions = (reminder.top_todo_sessions || []).slice(0, 8);
    const reminderBySession = new Map();
    taskSessions.forEach(item => {
        const session = item.session_id || 'unknown';
        const row = reminderBySession.get(session) || { session, label: shortAgentID(session), task: 0, todo: 0 };
        row.task = item.count || 0;
        reminderBySession.set(session, row);
    });
    todoSessions.forEach(item => {
        const session = item.session_id || 'unknown';
        const row = reminderBySession.get(session) || { session, label: shortAgentID(session), task: 0, todo: 0 };
        row.todo = item.count || 0;
        reminderBySession.set(session, row);
    });
    const reminderRows = Array.from(reminderBySession.values())
        .sort((a, b) => (b.task + b.todo) - (a.task + a.todo))
        .slice(0, 8);

    chart.setOption({
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'shadow',
                shadowStyle: { color: 'rgba(47, 128, 209, 0.08)' }
            },
            formatter: function(params) {
                return params
                    .filter(item => Number(item.value || 0) > 0)
                    .map(item => `${escapeHtml(item.seriesName)}: ${formatNumber(item.value)}`)
                    .join('<br/>') || '暂无数据';
            }
        },
        grid: [
            { top: 44, left: 140, right: 58, height: 150, containLabel: false },
            { top: 260, left: 180, right: 58, height: 190, containLabel: false },
            { top: 520, left: 150, right: 58, height: 150, containLabel: false }
        ],
        xAxis: [
            makeTaskPlanValueAxis(0),
            makeTaskPlanValueAxis(1),
            makeTaskPlanValueAxis(2)
        ],
        yAxis: [
            makeTaskPlanCategoryAxis(0, overviewRows.map(row => row.label), 122),
            makeTaskPlanCategoryAxis(1, fileRows.map(row => row.label), 162),
            makeTaskPlanCategoryAxis(2, reminderRows.map(row => row.label), 132)
        ],
        series: [
            makeTaskPlanBarSeries('次数', 0, overviewRows.map(row => ({
                value: row.value,
                itemStyle: { color: row.color }
            }))),
            makeTaskPlanBarSeries('引用次数', 1, fileRows.map(row => row.value), '#7c3aed'),
            makeTaskPlanBarSeries('Task Reminder', 2, reminderRows.map(row => row.task), '#d97706', 'reminder'),
            makeTaskPlanBarSeries('Todo Reminder', 2, reminderRows.map(row => row.todo), '#0f9f77', 'reminder')
        ],
        graphic: [
            makeChartSectionLabel('生命周期与任务状态', 0, 18),
            makeChartSectionLabel('计划文件引用 Top', 0, 234),
            makeChartSectionLabel('提醒频率 Top', 0, 494)
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

function makeTaskPlanValueAxis(gridIndex) {
    return {
        type: 'value',
        gridIndex,
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: {
            color: '#8a95a3',
            formatter: value => formatNumber(value)
        },
        splitLine: { lineStyle: { color: '#edf2f7', type: 'dashed' } }
    };
}

function makeTaskPlanCategoryAxis(gridIndex, labels, width) {
    return {
        type: 'category',
        gridIndex,
        data: labels.length > 0 ? labels : ['暂无数据'],
        inverse: true,
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: {
            color: '#4b5563',
            interval: 0,
            overflow: 'truncate',
            width
        }
    };
}

function makeTaskPlanBarSeries(name, axisIndex, data, color, stack) {
    return {
        name,
        type: 'bar',
        stack,
        xAxisIndex: axisIndex,
        yAxisIndex: axisIndex,
        data,
        barMaxWidth: 18,
        label: {
            show: true,
            position: 'right',
            color: '#596579',
            fontSize: 11,
            formatter: value => Number(value.value || 0) > 0 ? formatNumber(value.value) : ''
        },
        itemStyle: {
            color,
            borderRadius: [0, 4, 4, 0]
        }
    };
}

function makeChartSectionLabel(text, left, top) {
    return {
        type: 'text',
        left,
        top,
        style: {
            text,
            fill: '#1f2933',
            fontSize: 12,
            fontWeight: 650
        }
    };
}

// === M5: Tool Performance & Quality Analysis ===
function initToolPerformanceChart(tp) {
    var insight = document.getElementById('toolPerformanceChart-insight');
    if (!tp || !tp.by_category || tp.by_category.length === 0) {
        if (insight) insight.innerHTML = '<strong>数据洞察:</strong> 暂无工具性能数据';
        return;
    }

    var chart = echarts.init(document.getElementById('toolPerformanceChart'), 'wonderland');
    var categories = tp.by_category.slice(0, 20);
    var labels = categories.map(function(item) { return shortCategoryLabel(item.category); });
    var slowest = (tp.slowest_calls || []).slice(0, 10);
    var qualityDist = tp.quality_distribution || [];

    chart.setOption({
        title: {
            text: '工具性能与质量分析',
            subtext: '细分类别耗时 · 最慢调用 · 结果质量分布',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'shadow' },
            formatter: function(params) {
                var idx = params[0] ? params[0].dataIndex : -1;
                var item = categories[idx];
                if (!item) return '';
                return escapeHtml(item.category) + '<br/>' +
                    '调用: ' + formatNumber(item.call_count) + '<br/>' +
                    '成功: ' + formatNumber(item.success_count) + ' | 失败: ' + formatNumber(item.error_count) + ' | 缺失: ' + formatNumber(item.missing_count) + '<br/>' +
                    '失败率: ' + item.error_rate.toFixed(1) + '%<br/>' +
                    '总耗时: ' + formatDuration(item.total_duration_ms) + '<br/>' +
                    '平均耗时: ' + item.avg_duration_ms.toFixed(0) + 'ms<br/>' +
                    '最小: ' + formatDuration(item.min_duration_ms) + ' | 最大: ' + formatDuration(item.max_duration_ms) + '<br/>' +
                    '平均结果大小: ' + formatBytes(item.avg_result_size);
            }
        },
        legend: {
            data: ['总耗时', '平均耗时(ms)', '失败率%'],
            top: 50
        },
        grid: [
            { top: 95, left: 70, right: 70, height: 200, containLabel: true },
            { top: 330, left: 70, right: 70, height: 170, containLabel: true },
            { top: 530, left: 70, right: 70, height: 150, containLabel: true }
        ],
        xAxis: [
            { type: 'category', gridIndex: 0, data: labels, axisLabel: { interval: 0, rotate: 30, fontSize: 9 } },
            { type: 'category', gridIndex: 1, data: slowest.map(function(s) { return s.category || s.tool; }), axisLabel: { interval: 0, rotate: 25, fontSize: 9 } },
            { type: 'category', gridIndex: 2, data: qualityDist.map(function(q) { return q.bucket; }), axisLabel: { interval: 0, fontSize: 10 } }
        ],
        yAxis: [
            { type: 'value', gridIndex: 0, name: '耗时 ms' },
            { type: 'value', gridIndex: 0, name: '失败率 %', max: 100 },
            { type: 'value', gridIndex: 1, name: '耗时 ms' },
            { type: 'value', gridIndex: 2, name: '类别数' }
        ],
        series: [
            // Grid 0: Category performance - bar + lines
            {
                name: '总耗时', type: 'bar', xAxisIndex: 0, yAxisIndex: 0,
                data: categories.map(function(i) { return i.total_duration_ms; }),
                itemStyle: { color: '#5470c6' }
            },
            {
                name: '平均耗时(ms)', type: 'line', xAxisIndex: 0, yAxisIndex: 0,
                data: categories.map(function(i) { return +i.avg_duration_ms.toFixed(0); }),
                itemStyle: { color: '#91cc75' }, smooth: true
            },
            {
                name: '失败率%', type: 'line', xAxisIndex: 0, yAxisIndex: 1,
                data: categories.map(function(i) { return +i.error_rate.toFixed(1); }),
                itemStyle: { color: '#ee6666' }, smooth: true
            },
            // Grid 1: Slowest calls
            {
                name: '耗时(ms)', type: 'bar', xAxisIndex: 1, yAxisIndex: 2,
                data: slowest.map(function(s) { return s.duration_ms; }),
                itemStyle: function(params) {
                    return { color: slowest[params.dataIndex].is_error ? '#e74c3c' : '#f39c12' };
                }
            },
            // Grid 2: Quality distribution
            {
                name: '类别数', type: 'bar', xAxisIndex: 2, yAxisIndex: 3,
                data: qualityDist.map(function(q) { return q.count; }),
                itemStyle: { color: '#8e44ad' }
            }
        ]
    });

    // Insight text
    if (insight) {
        insight.innerHTML =
            '<strong>数据洞察:</strong> 共配对 <strong>' + formatNumber(tp.total_paired_calls) + '</strong> 次工具调用，' +
            '整体错误率 <strong>' + (tp.overall_error_rate || 0).toFixed(1) + '%</strong>，' +
            '平均耗时 <strong>' + (tp.overall_avg_duration || 0).toFixed(0) + 'ms</strong>。' +
            (slowest.length > 0 ? '最慢的单次调用是 <strong>' + escapeHtml(slowest[0].category || slowest[0].tool || '-') + '</strong>，耗时 <strong>' + formatDuration(slowest[0].duration_ms) + '</strong>。' : '');
    }
}

function shortCategoryLabel(cat) {
    if (!cat) return '-';
    if (cat.length > 35) return cat.slice(0, 32) + '...';
    return cat;
}

function formatBytes(val) {
    if (!val || val === 0) return '0 B';
    if (val < 1024) return val.toFixed(0) + ' B';
    if (val < 1048576) return (val / 1024).toFixed(1) + ' KB';
    return (val / 1048576).toFixed(1) + ' MB';
}

function formatDuration(ms) {
    if (!ms && ms !== 0) return '-';
    if (ms < 1000) return ms.toFixed(0) + 'ms';
    return (ms / 1000).toFixed(1) + 's';
}
