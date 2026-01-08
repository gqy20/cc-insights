// 当前时间范围
let currentPreset = 'all';

// 初始化
document.addEventListener('DOMContentLoaded', function() {
    setupEventListeners();
    loadData('all');
});

// 设置事件监听
function setupEventListeners() {
    // 预设按钮点击
    document.querySelectorAll('.preset-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            const preset = this.dataset.preset;
            setActivePreset(preset);
            loadData(`preset=${preset}`);
        });
    });

    // 设置默认日期
    const today = new Date().toISOString().split('T')[0];
    const weekAgo = new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
    document.getElementById('endDate').value = today;
    document.getElementById('startDate').value = weekAgo;
}

// 设置当前激活的预设
function setActivePreset(preset) {
    currentPreset = preset;
    document.querySelectorAll('.preset-btn').forEach(btn => {
        btn.classList.remove('active');
        if (btn.dataset.preset === preset) {
            btn.classList.add('active');
        }
    });
}

// 应用自定义范围
function applyCustomRange() {
    const startDate = document.getElementById('startDate').value;
    const endDate = document.getElementById('endDate').value;

    if (!startDate || !endDate) {
        showError('请选择开始和结束日期');
        return;
    }

    if (new Date(startDate) > new Date(endDate)) {
        showError('开始日期不能晚于结束日期');
        return;
    }

    // 清除预设按钮的激活状态
    document.querySelectorAll('.preset-btn').forEach(btn => {
        btn.classList.remove('active');
    });

    loadData(`preset=custom&start=${startDate}&end=${endDate}`);
}

// 加载数据
async function loadData(params) {
    showLoading(true);
    hideError();

    try {
        const response = await fetch(`/api/data?${params}`);
        const result = await response.json();

        if (!result.success) {
            throw new Error(result.error);
        }

        updateStatsInfo(result.data);
        renderCharts(result.data);

    } catch (error) {
        showError('加载数据失败: ' + error.message);
    } finally {
        showLoading(false);
    }
}

// 更新统计信息
function updateStatsInfo(data) {
    document.getElementById('lastUpdate').textContent = data.timestamp;

    let rangeText = '全部';
    if (data.time_range.preset === 'custom') {
        rangeText = `${data.time_range.start} 至 ${data.time_range.end}`;
    } else if (data.time_range.preset === '24h') {
        rangeText = '最近 24 小时';
    } else if (data.time_range.preset === '7d') {
        rangeText = '最近 7 天';
    } else if (data.time_range.preset === '30d') {
        rangeText = '最近 30 天';
    } else if (data.time_range.preset === '90d') {
        rangeText = '最近 90 天';
    }
    document.getElementById('rangeInfo').textContent = rangeText;

    const totalRecords = data.commands.reduce((sum, cmd) => sum + cmd.count, 0);
    document.getElementById('recordCount').textContent = totalRecords.toLocaleString();

    // 更新会话统计信息
    if (data.sessions) {
        const sessionInfo = document.getElementById('sessionInfo');
        if (sessionInfo) {
            sessionInfo.innerHTML = `
                <strong>总会话数:</strong> ${data.sessions.total_sessions.toLocaleString()} |
                <strong>峰值:</strong> ${data.sessions.peak_date} (${data.sessions.peak_count}) |
                <strong>谷值:</strong> ${data.sessions.valley_date} (${data.sessions.valley_count})
            `;
        }
    }
}

// 渲染图表
function renderCharts(data) {
    const container = document.getElementById('chartsContainer');
    container.innerHTML = '';

    // 每日趋势图
    container.appendChild(createChartDiv('dailyTrend', '1200px', '400px'));

    // 命令统计图
    container.appendChild(createChartDiv('commands', '1200px', '500px'));

    // MCP 工具图
    container.appendChild(createChartDiv('mcpTools', '900px', '700px'));

    // 会话统计图
    container.appendChild(createChartDiv('sessionChart', '1200px', '400px'));

    // 初始化 go-echarts 图表
    initDailyTrendChart(data.daily_trend);
    initCommandsChart(data.commands);
    initMCPToolsChart(data.mcp_tools);
    initSessionChart(data.sessions);

    container.style.display = 'block';
}

// 创建图表容器
function createChartDiv(id, width, height) {
    const wrapper = document.createElement('div');
    wrapper.className = 'chart-wrapper';
    wrapper.style.width = width;
    wrapper.style.marginBottom = '20px';

    const chartDiv = document.createElement('div');
    chartDiv.id = id;
    chartDiv.style.width = width;
    chartDiv.style.height = height;

    const insightDiv = document.createElement('div');
    insightDiv.id = `${id}-insight`;
    insightDiv.className = 'chart-insight';
    insightDiv.style.cssText = `
        margin-top: 15px;
        padding: 12px 15px;
        background: #f8f9fa;
        border-left: 4px solid #3498db;
        border-radius: 4px;
        font-size: 13px;
        line-height: 1.6;
        color: #555;
    `;

    wrapper.appendChild(chartDiv);
    wrapper.appendChild(insightDiv);
    return wrapper;
}

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
        `最常用的是 <strong>${topCmd.command}</strong>，使用了 <strong>${topCmd.count}</strong> 次（占比 ${topCmdPercent}%）。`;
}

// 初始化 MCP 工具图
function initMCPToolsChart(tools) {
    if (!tools || tools.length === 0) {
        document.getElementById('mcpTools-insight').innerHTML =
            '<strong>💡 数据洞察:</strong> 该时间范围内暂无 MCP 工具调用数据';
        return;
    }

    const chart = echarts.init(document.getElementById('mcpTools'), 'wonderland');

    const top10 = tools.slice(0, 10);
    const data = top10.map(t => ({
        name: `${t.server}::${t.tool}`,
        value: t.count
    }));

    const option = {
        title: {
            text: 'MCP 工具调用统计 (Top 10)',
            subtext: '数据来源: debug/ 目录',
            left: 'center',
            top: '20px'
        },
        tooltip: {
            trigger: 'item',
            formatter: '{b}: {c} ({d}%)'
        },
        series: [{
            name: 'MCP 工具调用',
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

    document.getElementById('mcpTools-insight').innerHTML =
        `<strong>💡 数据洞察:</strong> 共调用了 <strong>${tools.length}</strong> 种不同的 MCP 工具，` +
        `总计 <strong>${totalCalls.toLocaleString()}</strong> 次。` +
        `最活跃的服务器是 <strong>${topServer[0]}</strong>，最常用工具是 <strong>${topTool.server}::${topTool.tool}</strong>（占比 ${topToolPercent}%）。`;
}

// 显示/隐藏加载状态
function showLoading(show) {
    document.getElementById('loadingIndicator').style.display = show ? 'block' : 'none';
    document.getElementById('chartsContainer').style.display = show ? 'none' : 'flex';
}

// 显示错误
function showError(message) {
    const errorDiv = document.getElementById('errorMessage');
    errorDiv.innerHTML = `<div class="error">${message}</div>`;
}

// 隐藏错误
function hideError() {
    document.getElementById('errorMessage').innerHTML = '';
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
