// 初始化每日趋势图
function initDailyTrendChart(trendData) {
    if (!trendData || !trendData.counts || !trendData.dates || trendData.counts.length === 0) {
        document.getElementById('dailyTrend-insight').innerHTML =
            '<strong>💡 数据洞察:</strong> 该时间范围内暂无数据';
        return;
    }

    const chart = echarts.init(document.getElementById('dailyTrend'), 'wonderland');

    const option = {
        title: {
            text: '每日活动趋势',
            subtext: '数据来源: stats-cache.json',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis'
        },
        xAxis: {
            type: 'category',
            data: trendData.dates
        },
        yAxis: {
            type: 'value'
        },
        series: [{
            name: '消息数',
            type: 'line',
            data: trendData.counts,
            smooth: true,
            areaStyle: {
                opacity: 0.2
            }
        }]
    };

    chart.setOption(option);

    // 生成数据洞察
    const totalCount = trendData.counts.reduce((a, b) => a + b, 0);
    const avgCount = Math.round(totalCount / trendData.counts.length);
    const maxCount = Math.max(...trendData.counts);
    const maxIndex = trendData.counts.indexOf(maxCount);
    const peakDate = trendData.dates[maxIndex];

    document.getElementById('dailyTrend-insight').innerHTML =
        `<strong>💡 数据洞察:</strong> 统计期间共产生 <strong>${totalCount.toLocaleString()}</strong> 条消息，` +
        `日均 <strong>${avgCount.toLocaleString()}</strong> 条。` +
        `活动峰值在 <strong>${peakDate}</strong>，达到 <strong>${maxCount.toLocaleString()}</strong> 条消息。`;
}

// 初始化命令统计图
function initCommandsChart(commands) {
    if (!commands || commands.length === 0) {
        document.getElementById('commands-insight').innerHTML =
            '<strong>💡 数据洞察:</strong> 该时间范围内暂无命令数据';
        return;
    }

    const chart = echarts.init(document.getElementById('commands'), 'wonderland');

    const top15 = commands.slice(0, 15);

    const option = {
        title: {
            text: 'Slash Commands 使用统计 (Top 15)',
            subtext: '数据来源: history.jsonl',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'shadow'
            }
        },
        xAxis: {
            type: 'category',
            data: top15.map(c => c.command),
            axisLabel: {
                interval: 0,
                rotate: 45
            }
        },
        yAxis: {
            type: 'value'
        },
        series: [{
            name: '使用次数',
            type: 'bar',
            data: top15.map(c => ({ value: c.count })),
            label: {
                show: true,
                position: 'top'
            }
        }]
    };

    chart.setOption(option);

    // 生成数据洞察
    const totalCmds = commands.reduce((a, b) => a + b.count, 0);
    const topCmd = commands[0];
    const topCmdPercent = ((topCmd.count / totalCmds) * 100).toFixed(1);
    const uniqueCmds = commands.length;

    document.getElementById('commands-insight').innerHTML =
        `<strong>💡 数据洞察:</strong> 共使用了 <strong>${uniqueCmds}</strong> 种不同的命令，` +
        `总计 <strong>${totalCmds.toLocaleString()}</strong> 次。` +
        `最常用的是 <strong>${escapeHtml(topCmd.command)}</strong>，使用了 <strong>${topCmd.count}</strong> 次（占比 ${topCmdPercent}%）。`;
}

// 初始化 Runtime 工具图
function initRuntimeToolsChart(tools) {
    if (!tools || tools.length === 0) {
        document.getElementById('runtimeTools-insight').innerHTML =
            '<strong>💡 数据洞察:</strong> 该时间范围内暂无 Runtime 工具信号数据';
        return;
    }

    const chart = echarts.init(document.getElementById('runtimeTools'), 'wonderland');

    const top10 = tools.slice(0, 10);
    const data = top10.map(t => ({
        name: `${t.server}::${t.tool}`,
        value: t.count
    }));

    const option = {
        title: {
            text: 'Runtime 工具信号 (Top 10)',
            subtext: 'debug/ 日志中的 MCP 信号',
            left: 'center',
            top: '20px'
        },
        tooltip: {
            trigger: 'item',
            formatter: function(params) {
                return `${escapeHtml(params.name)}: ${formatNumber(params.value)} (${params.percent}%)`;
            }
        },
        series: [{
            name: 'Runtime 工具信号',
            type: 'pie',
            data: data,
            radius: '70%',
            label: {
                show: true,
                formatter: '{b}: {c}\n({d}%)'
            }
        }]
    };

    chart.setOption(option);

    // 生成数据洞察
    const totalCalls = tools.reduce((a, b) => a + b.count, 0);
    const topTool = tools[0];
    const topToolPercent = ((topTool.count / totalCalls) * 100).toFixed(1);
    const serverCounts = {};
    tools.forEach(t => {
        serverCounts[t.server] = (serverCounts[t.server] || 0) + t.count;
    });
    const topServer = Object.entries(serverCounts).sort((a, b) => b[1] - a[1])[0];

    document.getElementById('runtimeTools-insight').innerHTML =
        `<strong>💡 数据洞察:</strong> debug 日志捕捉到 <strong>${tools.length}</strong> 种 Runtime 工具信号，` +
        `总计 <strong>${totalCalls.toLocaleString()}</strong> 次。` +
        `最活跃的服务器是 <strong>${escapeHtml(topServer[0])}</strong>，最常用工具是 <strong>${escapeHtml(topTool.server)}::${escapeHtml(topTool.tool)}</strong>（占比 ${topToolPercent}%）。`;
}

// 显示/隐藏加载状态
function showLoading(show, preset = 'all') {
    const loadingIndicator = document.getElementById('loadingIndicator');
    const chartsContainer = document.getElementById('chartsContainer');

    if (show) {
        // 显示加载动画
        loadingIndicator.style.display = 'block';
        chartsContainer.style.display = 'none';

        // 设置预估时间
        const eta = getEstimatedTime(preset);
        const etaEl = document.getElementById('loadingEta');
        if (etaEl) {
            etaEl.textContent = `预计需要 ${eta.min}-${eta.max} 秒`;
        }

        // 设置随机趣味文案
        const tipEl = document.getElementById('loadingTip');
        if (tipEl) {
            tipEl.textContent = getRandomTip();
        }

        // 重置进度文本
        updateLoadingProgress(loadingStages[0]);
    } else {
        // 隐藏加载动画
        loadingIndicator.style.display = 'none';
        chartsContainer.style.display = '';
    }
}

// 显示错误
function showError(message) {
    const errorDiv = document.getElementById('errorMessage');
    const messageDiv = document.createElement('div');
    messageDiv.className = 'error';
    messageDiv.textContent = message;
    errorDiv.replaceChildren(messageDiv);
}

// 隐藏错误
function hideError() {
    document.getElementById('errorMessage').replaceChildren();
}

// 初始化会话统计图
function initSessionChart(sessionData) {
    if (!sessionData || !sessionData.daily_session_map || Object.keys(sessionData.daily_session_map).length === 0) {
        document.getElementById('sessionChart-insight').innerHTML =
            '<strong>💡 数据洞察:</strong> 该时间范围内暂无会话数据';
        return;
    }

    const chart = echarts.init(document.getElementById('sessionChart'), 'wonderland');

    // 将 map 转换为数组并按日期排序
    const dates = Object.keys(sessionData.daily_session_map).sort();
    const counts = dates.map(d => sessionData.daily_session_map[d]);

    const option = {
        title: {
            text: '每日会话趋势',
            subtext: `总计: ${sessionData.total_sessions.toLocaleString()} 次会话 | 峰值: ${sessionData.peak_date} (${sessionData.peak_count}) | 谷值: ${sessionData.valley_date} (${sessionData.valley_count})`,
            left: 'center'
        },
        tooltip: {
            trigger: 'axis'
        },
        xAxis: {
            type: 'category',
            data: dates,
            axisLabel: {
                interval: 0,
                rotate: 45
            }
        },
        yAxis: {
            type: 'value',
            name: '会话数'
        },
        series: [{
            name: '会话数',
            type: 'line',
            data: counts,
            smooth: true,
            areaStyle: {
                opacity: 0.2
            },
            markPoint: {
                data: [
                    { type: 'max', name: '峰值' },
                    { type: 'min', name: '谷值' }
                ]
            },
            label: {
                show: false
            }
        }]
    };

    chart.setOption(option);

    // 生成数据洞察
    const avgSessions = Math.round(sessionData.total_sessions / dates.length);
    const peakValleyRatio = (sessionData.peak_count / sessionData.valley_count).toFixed(1);

    document.getElementById('sessionChart-insight').innerHTML =
        `<strong>💡 数据洞察:</strong> 统计期间共创建 <strong>${sessionData.total_sessions.toLocaleString()}</strong> 个会话，` +
        `日均 <strong>${avgSessions.toLocaleString()}</strong> 个。` +
        `峰值日 <strong>${sessionData.peak_date}</strong> 的会话数是谷值日 <strong>${sessionData.valley_date}</strong> 的 <strong>${peakValleyRatio}</strong> 倍。`;
}

// 初始化项目活跃度排名图
function initProjectChart(projectData) {
    if (!projectData || !projectData.projects || projectData.projects.length === 0) {
        document.getElementById('projectChart-insight').innerHTML =
            '<strong>💡 数据洞察:</strong> 该时间范围内暂无项目数据';
        return;
    }

    const chart = echarts.init(document.getElementById('projectChart'), 'wonderland');

    // 取 Top 15 项目，并简化项目名显示
    const top15 = projectData.projects.slice(0, 15).map(p => ({
        name: p.project.split('/').pop() || p.project,
        value: p.message_count,
        originalName: p.project
    }));

    const option = {
        title: {
            text: '项目活跃度排名 (Top 15)',
            subtext: '数据来源: projects/*.jsonl',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'shadow'
            },
            formatter: function(params) {
                const item = top15[params[0].dataIndex];
                return `${escapeHtml(item.originalName)}<br/>消息数: ${item.value.toLocaleString()}`;
            }
        },
        xAxis: {
            type: 'category',
            data: top15.map(p => p.name),
            axisLabel: {
                interval: 0,
                rotate: 45
            }
        },
        yAxis: {
            type: 'value',
            name: '消息数'
        },
        series: [{
            name: '消息数',
            type: 'bar',
            data: top15.map(p => p.value),
            label: {
                show: true,
                position: 'top',
                formatter: function(v) {
                    return v.data.toLocaleString();
                }
            }
        }]
    };

    chart.setOption(option);

    // 生成数据洞察
    const topProject = top15[0];
    const topPercent = ((topProject.value / projectData.total_messages) * 100).toFixed(1);

    document.getElementById('projectChart-insight').innerHTML =
        `<strong>💡 数据洞察:</strong> 统计期间共涉及 <strong>${projectData.projects.length}</strong> 个项目，` +
        `总计 <strong>${projectData.total_messages.toLocaleString()}</strong> 条消息。` +
        `最活跃的是 <strong>${escapeHtml(topProject.name)}</strong>，贡献了 ${topPercent}% 的消息。`;
}

// 初始化星期分布图
function initWeekdayChart(weekdayData) {
    if (!weekdayData || !weekdayData.weekday_data || weekdayData.weekday_data.length === 0) {
        document.getElementById('weekdayChart-insight').innerHTML =
            '<strong>💡 数据洞察:</strong> 该时间范围内暂无星期数据';
        return;
    }

    const chart = echarts.init(document.getElementById('weekdayChart'), 'wonderland');

    const weekdays = weekdayData.weekday_data;
    const maxCount = Math.max(...weekdays.map(w => w.message_count));
    const maxWeekday = weekdays.find(w => w.message_count === maxCount);
    const totalMessages = weekdays.reduce((sum, w) => sum + w.message_count, 0);

    const option = {
        title: {
            text: '星期活动分布',
            subtext: '数据来源: projects/*.jsonl',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis',
            formatter: function(params) {
                const item = params[0];
                const count = Number(item.value || 0);
                const share = totalMessages > 0 ? (count / totalMessages * 100).toFixed(1) : '0.0';
                return `${escapeHtml(item.axisValue)}<br/>消息数: ${formatNumber(count)}<br/>占当前范围: ${share}%`;
            }
        },
        xAxis: {
            type: 'category',
            data: weekdays.map(w => w.weekday_name)
        },
        yAxis: {
            type: 'value',
            name: '消息数'
        },
        series: [{
            name: '消息数',
            type: 'bar',
            data: weekdays.map(w => w.message_count),
            smooth: true,
            label: {
                show: true,
                position: 'top',
                formatter: function(v) {
                    return formatNumber(v.data);
                }
            },
            itemStyle: {
                color: function(params) {
                    // 高亮星期天（周末）
                    const colors = ['#5470c6', '#5470c6', '#5470c6', '#5470c6', '#5470c6', '#91cc75', '#91cc75'];
                    return colors[params.dataIndex];
                }
            }
        }]
    };

    chart.setOption(option);

    // 生成数据洞察
    const avgMessages = Math.round(totalMessages / 7);
    const workdayTotal = weekdays.slice(0, 5).reduce((sum, w) => sum + w.message_count, 0);
    const weekendTotal = weekdays.slice(5).reduce((sum, w) => sum + w.message_count, 0);

    document.getElementById('weekdayChart-insight').innerHTML =
        `<strong>💡 数据洞察:</strong> 当前范围共 <strong>${formatNumber(totalMessages)}</strong> 条消息。` +
        `最活跃的是 <strong>${escapeHtml(maxWeekday.weekday_name)}</strong>（${formatNumber(maxCount)} 条），` +
        `日均 <strong>${formatNumber(avgMessages)}</strong> 条。` +
        `工作日 <strong>${formatNumber(workdayTotal)}</strong> 条，周末 <strong>${formatNumber(weekendTotal)}</strong> 条。`;
}

// 初始化模型使用分析图
