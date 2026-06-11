const chartInstances = new Set();
let resizeTimer = null;

const chartDefinitions = [
    { id: 'dailyTrend', height: 400, layout: 'wide', render: data => initDailyTrendChart(data.daily_trend) },
    { id: 'commands', height: 500, layout: 'compact', render: data => initCommandsChart(data.commands) },
    { id: 'mcpTools', height: 700, layout: 'compact', render: data => initMCPToolsChart(data.mcp_tools) },
    { id: 'sessionChart', height: 400, layout: 'wide', render: data => initSessionChart(data.sessions) },
    { id: 'projectChart', height: 500, layout: 'compact', render: data => initProjectChart(data.project_stats) },
    { id: 'weekdayChart', height: 400, layout: 'compact', render: data => initWeekdayChart(data.weekday_stats) },
    { id: 'modelChart', height: 500, layout: 'compact', render: data => initModelChart(data.model_usage) },
    { id: 'workHoursChart', height: 450, layout: 'compact', render: data => initWorkHoursChart(data.work_hours_stats) },
    { id: 'toolModelFailureChart', height: 560, layout: 'wide', render: data => initToolModelFailureChart(data.tool_analysis) },
    { id: 'failureReasonChart', height: 620, layout: 'wide', render: data => initFailureReasonChart(data.failure_analysis) },
    { id: 'eventHookChart', height: 560, layout: 'compact', render: data => initEventHookChart(data.event_analysis) },
    { id: 'agentChart', height: 560, layout: 'compact', render: data => initAgentChart(data.agent_analysis) },
    { id: 'commandFileChart', height: 620, layout: 'wide', render: data => initCommandFileChart(data.command_analysis) },
    { id: 'costAnalysisChart', height: 620, layout: 'wide', render: data => initCostAnalysisChart(data.cost_analysis) },
    { id: 'sessionAnalysisChart', height: 620, layout: 'wide', render: data => initSessionAnalysisChart(data.session_analysis) },
    { id: 'fileAnalysisChart', height: 700, layout: 'wide', render: data => initFileAnalysisChart(data.file_analysis) },
    { id: 'taskPlanChart', height: 750, layout: 'wide', render: data => initTaskPlanChart(data.task_plan_analysis) },
    { id: 'toolPerformanceChart', height: 800, layout: 'wide', render: data => initToolPerformanceChart(data.tool_performance) }
];

function installChartTracking() {
    if (!window.echarts || window.echarts.__ccInsightsTracked) {
        return;
    }

    const originalInit = window.echarts.init.bind(window.echarts);
    window.echarts.init = function(dom, theme, opts) {
        const existing = window.echarts.getInstanceByDom(dom);
        if (existing) {
            existing.dispose();
            chartInstances.delete(existing);
        }

        const chart = originalInit(dom, theme, opts);
        chartInstances.add(chart);
        return chart;
    };
    window.echarts.__ccInsightsTracked = true;
}

function disposeCharts() {
    chartInstances.forEach(chart => {
        if (chart && !chart.isDisposed()) {
            chart.dispose();
        }
    });
    chartInstances.clear();
}

function resizeCharts() {
    chartInstances.forEach(chart => {
        if (chart && !chart.isDisposed()) {
            chart.resize();
        }
    });
}

function setupChartResizeListener() {
    window.addEventListener('resize', () => {
        clearTimeout(resizeTimer);
        resizeTimer = setTimeout(resizeCharts, 120);
    });
}

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
            sessionInfo.textContent =
                `总会话数: ${data.sessions.total_sessions.toLocaleString()} | ` +
                `峰值: ${data.sessions.peak_date} (${data.sessions.peak_count}) | ` +
                `谷值: ${data.sessions.valley_date} (${data.sessions.valley_count})`;
        }
    }
}

// 渲染图表
function renderCharts(data) {
    const container = document.getElementById('chartsContainer');
    disposeCharts();
    container.replaceChildren();

    chartDefinitions.forEach(definition => {
        container.appendChild(createChartDiv(definition));
    });

    container.style.display = '';

    chartDefinitions.forEach(definition => {
        definition.render(data);
    });

    requestAnimationFrame(resizeCharts);
}

// 创建图表容器
function createChartDiv(definition) {
    const wrapper = document.createElement('div');
    wrapper.className = `chart-wrapper ${definition.layout === 'wide' ? 'wide' : 'compact'}`;

    const chartDiv = document.createElement('div');
    chartDiv.id = definition.id;
    chartDiv.className = 'chart-canvas';
    chartDiv.style.setProperty('--chart-height', `${definition.height}px`);

    const insightDiv = document.createElement('div');
    insightDiv.id = `${definition.id}-insight`;
    insightDiv.className = 'chart-insight';

    wrapper.appendChild(chartDiv);
    wrapper.appendChild(insightDiv);
    return wrapper;
}
