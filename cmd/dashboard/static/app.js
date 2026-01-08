// 当前时间范围
let currentPreset = 'all';

// 趣味加载文案
const loadingTips = [
    "☕ 顺便喝口水吧~",
    "📊 正在整理您的数据碎片...",
    "🤖 正在向 Claude 询问您的使用习惯...",
    "⏳ 数据有点多，给我几秒钟...",
    "🎯 稍安勿躁，精彩即将呈现",
    "💡 您的每一次使用都被记录了下来",
    "🚀 让我们一起看看您的生产力",
    "📈 数据正在转化为洞察...",
    "🌟 感谢您使用 Claude Code",
    "🎨 准备绘制您的使用图表"
];

// 加载阶段提示
const loadingStages = [
    "正在读取数据文件...",
    "正在解析历史记录...",
    "正在分析 MCP 工具调用...",
    "正在生成图表...",
    "即将完成..."
];

// 获取随机趣味文案
function getRandomTip() {
    return loadingTips[Math.floor(Math.random() * loadingTips.length)];
}

// 获取预估时间（秒）
function getEstimatedTime(preset) {
    const estimates = {
        '24h': { min: 1, max: 2 },
        '7d': { min: 2, max: 3 },
        '30d': { min: 5, max: 8 },
        '90d': { min: 10, max: 15 },
        'all': { min: 10, max: 20 },
        'custom': { min: 3, max: 6 }
    };
    return estimates[preset] || estimates['all'];
}

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
    // 从参数中解析预设
    const urlParams = new URLSearchParams(params);
    const preset = urlParams.get('preset') || 'all';

    showLoading(true, preset);
    hideError();

    // 分阶段更新加载提示
    let stageIndex = 0;
    const stageInterval = setInterval(() => {
        if (stageIndex < loadingStages.length) {
            updateLoadingProgress(loadingStages[stageIndex]);
            stageIndex++;
        }
    }, 800); // 每800毫秒更新一次阶段

    try {
        const response = await fetch(`/api/data?${params}`);
        const result = await response.json();

        // 停止阶段更新
        clearInterval(stageInterval);

        // 显示最后阶段
        updateLoadingProgress(loadingStages[loadingStages.length - 1]);

        if (!result.success) {
            throw new Error(result.error);
        }

        // 短暂延迟以显示"即将完成"
        await new Promise(resolve => setTimeout(resolve, 300));

        updateStatsInfo(result.data);
        renderCharts(result.data);

    } catch (error) {
        clearInterval(stageInterval);
        showError('加载数据失败: ' + error.message);
    } finally {
        showLoading(false);
    }
}

// 更新加载进度文本
function updateLoadingProgress(text) {
    const progressEl = document.getElementById('loadingProgress');
    if (progressEl) {
        progressEl.textContent = text;
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

    // 项目活跃度排名图
    container.appendChild(createChartDiv('projectChart', '1200px', '500px'));

    // 星期分布图
    container.appendChild(createChartDiv('weekdayChart', '1000px', '400px'));

    // 模型使用分析图
    container.appendChild(createChartDiv('modelChart', '1000px', '500px'));

    // 初始化 go-echarts 图表
    initDailyTrendChart(data.daily_trend);
    initCommandsChart(data.commands);
    initMCPToolsChart(data.mcp_tools);
    initSessionChart(data.sessions);
    initProjectChart(data.project_stats);
    initWeekdayChart(data.weekday_stats);
    initModelChart(data.model_usage);

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
        chartsContainer.style.display = 'flex';
    }
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
                return `${item.originalName}<br/>消息数: ${item.value.toLocaleString()}`;
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
        `最活跃的是 <strong>${topProject.name}</strong>，贡献了 ${topPercent}% 的消息。`;
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

    const option = {
        title: {
            text: '星期活动分布',
            subtext: '数据来源: projects/*.jsonl',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis'
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
                    return v.data.toLocaleString();
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
    const totalMessages = weekdays.reduce((sum, w) => sum + w.message_count, 0);
    const avgMessages = Math.round(totalMessages / 7);
    const workdayTotal = weekdays.slice(0, 5).reduce((sum, w) => sum + w.message_count, 0);
    const weekendTotal = weekdays.slice(5).reduce((sum, w) => sum + w.message_count, 0);

    document.getElementById('weekdayChart-insight').innerHTML =
        `<strong>💡 数据洞察:</strong> 最活跃的是 <strong>${maxWeekday.weekday_name}</strong>（${maxCount.toLocaleString()} 条消息），` +
        `日均 <strong>${avgMessages.toLocaleString()}</strong> 条。` +
        `工作日共 <strong>${workdayTotal.toLocaleString()}</strong> 条，周末 <strong>${weekendTotal.toLocaleString()}</strong> 条。`;
}

// 初始化模型使用分析图
function initModelChart(modelData) {
    if (!modelData || modelData.length === 0) {
        document.getElementById('modelChart-insight').innerHTML =
            '<strong>💡 数据洞察:</strong> 该时间范围内暂无模型使用数据';
        return;
    }

    const chart = echarts.init(document.getElementById('modelChart'), 'wonderland');

    const models = modelData.map(m => m.model);
    const counts = modelData.map(m => m.count);
    const tokens = modelData.map(m => m.tokens);

    const maxCount = Math.max(...counts);
    const topModel = modelData[0];
    const totalRequests = counts.reduce((sum, c) => sum + c, 0);
    const totalTokens = tokens.reduce((sum, t) => sum + t, 0);
    const avgTokensPerRequest = Math.round(totalTokens / totalRequests);

    const option = {
        title: {
            text: '模型使用分析',
            subtext: '数据来源: projects/*.jsonl',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'cross'
            }
        },
        legend: {
            data: ['请求数', 'Token数'],
            top: 30
        },
        xAxis: {
            type: 'category',
            data: models,
            axisLabel: {
                interval: 0,
                rotate: models.length > 4 ? 30 : 0,
                formatter: function(value) {
                    // 简化模型名称显示
                    return value.length > 20 ? value.substring(0, 20) + '...' : value;
                }
            }
        },
        yAxis: [
            {
                type: 'value',
                name: '请求数',
                position: 'left'
            },
            {
                type: 'value',
                name: 'Token数',
                position: 'right'
            }
        ],
        series: [
            {
                name: '请求数',
                type: 'bar',
                data: counts,
                label: {
                    show: true,
                    position: 'top',
                    formatter: function(v) {
                        return v.data.toLocaleString();
                    }
                },
                itemStyle: {
                    color: '#5470c6'
                }
            },
            {
                name: 'Token数',
                type: 'line',
                yAxisIndex: 1,
                data: tokens,
                smooth: true,
                itemStyle: {
                    color: '#91cc75'
                },
                lineStyle: {
                    width: 2
                }
            }
        ]
    };

    chart.setOption(option);

    // 生成数据洞察
    const topModelShare = ((topModel.count / totalRequests) * 100).toFixed(1);

    document.getElementById('modelChart-insight').innerHTML =
        `<strong>💡 数据洞察:</strong> 最常用的是 <strong>${topModel.model}</strong>（${topModel.count.toLocaleString()} 次请求，占比 ${topModelShare}%），` +
        `总计 <strong>${totalRequests.toLocaleString()}</strong> 次请求，` +
        `消耗 <strong>${(totalTokens / 1000000).toFixed(1)}M</strong> Tokens，` +
        `平均每次请求 <strong>${avgTokensPerRequest.toLocaleString()}</strong> Tokens。`;
}
