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
            loadData(preset);
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

    loadData(`custom?start=${startDate}&end=${endDate}`);
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
}

// 渲染图表
function renderCharts(data) {
    const container = document.getElementById('chartsContainer');
    container.innerHTML = '';

    // 每日趋势图
    container.appendChild(createChartDiv('dailyTrend', '1200px', '400px'));

    // 命令统计图
    container.appendChild(createChartDiv('commands', '1200px', '500px'));

    // 小时分布图
    container.appendChild(createChartDiv('hourly', '1200px', '400px'));

    // MCP 工具图
    container.appendChild(createChartDiv('mcpTools', '900px', '700px'));

    // 初始化 go-echarts 图表
    initDailyTrendChart(data.daily_trend);
    initCommandsChart(data.commands);
    initHourlyChart(data.hourly_counts);
    initMCPToolsChart(data.mcp_tools);

    container.style.display = 'block';
}

// 创建图表容器
function createChartDiv(id, width, height) {
    const div = document.createElement('div');
    div.className = 'chart-wrapper';
    div.id = id;
    div.style.width = width;
    div.style.height = height;
    return div;
}

// 初始化每日趋势图
function initDailyTrendChart(trendData) {
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
            },
            label: {
                show: true
            }
        }]
    };

    chart.setOption(option);
}

// 初始化命令统计图
function initCommandsChart(commands) {
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
}

// 初始化小时分布图
function initHourlyChart(hourlyCounts) {
    const chart = echarts.init(document.getElementById('hourly'), 'wonderland');

    const hours = Array.from({length: 24}, (_, i) => `${String(i).padStart(2, '0')}:00`);
    const counts = hours.map(h => hourlyCounts[h.replace(':00', '')] || 0);

    const option = {
        title: {
            text: '24小时活动分布',
            subtext: '数据来源: history.jsonl',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis'
        },
        xAxis: {
            type: 'category',
            data: hours
        },
        yAxis: {
            type: 'value'
        },
        series: [{
            name: '活动次数',
            type: 'bar',
            data: counts
        }]
    };

    chart.setOption(option);
}

// 初始化 MCP 工具图
function initMCPToolsChart(tools) {
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
