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
    container.appendChild(createChartDiv('mcpTools', '1200px', '700px'));

    // 会话统计图
    container.appendChild(createChartDiv('sessionChart', '1200px', '400px'));

    // 项目活跃度排名图
    container.appendChild(createChartDiv('projectChart', '1200px', '500px'));

    // 星期分布图
    container.appendChild(createChartDiv('weekdayChart', '1200px', '400px'));

    // 模型使用分析图
    container.appendChild(createChartDiv('modelChart', '1200px', '500px'));

    // 工作时段热力图
    container.appendChild(createChartDiv('workHoursChart', '1200px', '450px'));

    // 工具失败与模型关联
    container.appendChild(createChartDiv('toolModelFailureChart', '1200px', '560px'));

    // 失败原因细分
    container.appendChild(createChartDiv('failureReasonChart', '1200px', '620px'));

    // 运行事件与 Hook 状态
    container.appendChild(createChartDiv('eventHookChart', '1200px', '560px'));

    // Agent / Subagent 工具调用
    container.appendChild(createChartDiv('agentChart', '1200px', '560px'));

    // Bash 与文件操作失败
    container.appendChild(createChartDiv('commandFileChart', '1200px', '620px'));

    // Token 与成本分析
    container.appendChild(createChartDiv('costAnalysisChart', '1200px', '620px'));

    // Session 生命周期复盘
    container.appendChild(createChartDiv('sessionAnalysisChart', '1200px', '620px'));

    // 文件与编辑质量分析
    container.appendChild(createChartDiv('fileAnalysisChart', '1200px', '700px'));

    // Task / Plan 结构分析（M4）
    container.appendChild(createChartDiv('taskPlanChart', '1200px', '750px'));

    // 初始化 go-echarts 图表
    initDailyTrendChart(data.daily_trend);
    initCommandsChart(data.commands);
    initMCPToolsChart(data.mcp_tools);
    initSessionChart(data.sessions);
    initProjectChart(data.project_stats);
    initWeekdayChart(data.weekday_stats);
    initModelChart(data.model_usage);
    initWorkHoursChart(data.work_hours_stats);
    initToolModelFailureChart(data.tool_analysis);
    initFailureReasonChart(data.failure_analysis);
    initEventHookChart(data.event_analysis);
    initAgentChart(data.agent_analysis);
    initCommandFileChart(data.command_analysis);
    initCostAnalysisChart(data.cost_analysis);
    initSessionAnalysisChart(data.session_analysis);
    initFileAnalysisChart(data.file_analysis);
    initTaskPlanChart(data.task_plan_analysis);

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
