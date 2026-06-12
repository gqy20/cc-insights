const chartInstances = new Set();
let resizeTimer = null;

const chartGroups = [
    { id: 'usage', title: '使用', description: '命令、工具、模型和工作时段分布。' },
    { id: 'quality', title: '质量', description: '失败、缺失结果、文件编辑质量和工具健康度。' },
    { id: 'cost', title: '成本', description: 'Token、模型、项目和会话级成本信号。' },
    { id: 'runtime', title: '运行时', description: 'Session 生命周期、Agent、Task/Plan 和运行事件。' }
];

const chartDefinitions = [
    {
        id: 'dailyTrend',
        group: 'usage',
        title: '每日活动趋势',
        description: '按日期汇总消息量，用于识别活跃峰值和最近走势。',
        height: 400,
        layout: 'wide',
        hasData: data => hasArrayData(data.daily_trend && data.daily_trend.counts),
        emptyText: '该时间范围内没有活动消息。',
        render: data => initDailyTrendChart(data.daily_trend)
    },
    {
        id: 'commands',
        group: 'usage',
        title: 'Slash Commands',
        description: '统计常用命令和调用占比，判断日常工作入口。',
        height: 500,
        layout: 'compact',
        hasData: data => hasArrayData(data.commands),
        emptyText: '该时间范围内没有 Slash Command 调用。',
        render: data => initCommandsChart(data.commands)
    },
    {
        id: 'mcpTools',
        group: 'usage',
        title: 'MCP 工具调用',
        description: '按 server 和 tool 汇总外部工具使用频率。',
        height: 700,
        layout: 'compact',
        hasData: data => hasArrayData(data.mcp_tools),
        emptyText: '该时间范围内没有 MCP 工具调用。',
        render: data => initMCPToolsChart(data.mcp_tools)
    },
    {
        id: 'projectChart',
        group: 'usage',
        title: '项目活跃度',
        description: '展示项目维度的消息量和会话活跃排名。',
        height: 500,
        layout: 'compact',
        hasData: data => hasArrayData(data.project_stats && data.project_stats.projects),
        emptyText: '该时间范围内没有项目活跃数据。',
        render: data => initProjectChart(data.project_stats)
    },
    {
        id: 'weekdayChart',
        group: 'usage',
        title: '星期活动分布',
        description: '按星期聚合消息量，观察固定工作节奏。',
        height: 400,
        layout: 'compact',
        hasData: data => sumBy(data.weekday_stats && data.weekday_stats.weekday_data, 'message_count') > 0,
        emptyText: '该时间范围内没有可展示的星期分布。',
        render: data => initWeekdayChart(data.weekday_stats)
    },
    {
        id: 'modelChart',
        group: 'usage',
        title: '模型使用',
        description: '对比不同模型的请求量、Token 和使用占比。',
        height: 500,
        layout: 'compact',
        hasData: data => hasArrayData(data.model_usage),
        emptyText: '该时间范围内没有模型使用数据。',
        render: data => initModelChart(data.model_usage)
    },
    {
        id: 'workHoursChart',
        group: 'usage',
        title: '工作时段',
        description: '按小时统计活动，定位高峰时段和离散工作模式。',
        height: 450,
        layout: 'compact',
        hasData: data => sumBy(data.work_hours_stats && data.work_hours_stats.hourly_data, 'count') > 0,
        emptyText: '该时间范围内没有可展示的工作时段分布。',
        render: data => initWorkHoursChart(data.work_hours_stats)
    },
    {
        id: 'toolModelFailureChart',
        group: 'quality',
        title: '模型 × 工具失败',
        description: '交叉分析模型与工具的失败、缺失结果和失败率。',
        height: 560,
        layout: 'wide',
        hasData: data => hasArrayData(data.tool_analysis && data.tool_analysis.by_model),
        emptyText: '该时间范围内没有工具失败或缺失结果关联。',
        state: 'healthy',
        render: data => initToolModelFailureChart(data.tool_analysis)
    },
    {
        id: 'failureReasonChart',
        group: 'quality',
        title: '失败原因',
        description: '按原因、工具和模型拆解失败来源。',
        height: 620,
        layout: 'wide',
        hasData: data => hasArrayData(data.failure_analysis && data.failure_analysis.by_reason),
        emptyText: '该时间范围内没有失败原因细分数据。',
        state: 'healthy',
        render: data => initFailureReasonChart(data.failure_analysis)
    },
    {
        id: 'commandFileChart',
        group: 'quality',
        title: 'Bash 与文件操作',
        description: '查看命令、文件操作和异常退出的集中位置。',
        height: 620,
        layout: 'wide',
        hasData: data => hasArrayData(data.command_analysis && data.command_analysis.bash_commands) || hasArrayData(data.command_analysis && data.command_analysis.file_operations),
        emptyText: '该时间范围内没有 Bash 或文件操作异常。',
        state: 'healthy',
        render: data => initCommandFileChart(data.command_analysis)
    },
    {
        id: 'fileAnalysisChart',
        group: 'quality',
        title: '文件编辑质量',
        description: '聚合热门文件、编辑失败和快照热点。',
        height: 700,
        layout: 'wide',
        hasData: data => Boolean(data.file_analysis && data.file_analysis.totals && data.file_analysis.totals.unique_files > 0),
        emptyText: '该时间范围内没有文件编辑质量数据。',
        render: data => initFileAnalysisChart(data.file_analysis)
    },
    {
        id: 'toolPerformanceChart',
        group: 'quality',
        title: '工具性能',
        description: '比较工具耗时、错误率和结果完整度。',
        height: 800,
        layout: 'wide',
        hasData: data => hasArrayData(data.tool_performance && data.tool_performance.by_category),
        emptyText: '该时间范围内没有工具性能数据。',
        render: data => initToolPerformanceChart(data.tool_performance)
    },
    {
        id: 'costAnalysisChart',
        group: 'cost',
        title: 'Token 与成本',
        description: '从模型、项目和会话维度追踪消耗结构。',
        height: 620,
        layout: 'wide',
        hasData: data => Boolean(data.cost_analysis && data.cost_analysis.totals && data.cost_analysis.totals.total_tokens > 0),
        emptyText: '该时间范围内没有 Token 或成本数据。',
        render: data => initCostAnalysisChart(data.cost_analysis)
    },
    {
        id: 'sessionChart',
        group: 'runtime',
        title: '每日会话趋势',
        description: '按日期展示会话数量变化和峰谷表现。',
        height: 400,
        layout: 'wide',
        hasData: data => Boolean(data.sessions && data.sessions.total_sessions > 0),
        emptyText: '该时间范围内没有会话数据。',
        render: data => initSessionChart(data.sessions)
    },
    {
        id: 'eventHookChart',
        group: 'runtime',
        title: '运行事件与 Hook',
        description: '汇总事件类型、Hook 状态和运行时反馈。',
        height: 560,
        layout: 'compact',
        hasData: data => hasArrayData(data.event_analysis && data.event_analysis.by_type),
        emptyText: '该时间范围内没有运行事件数据。',
        render: data => initEventHookChart(data.event_analysis)
    },
    {
        id: 'agentChart',
        group: 'runtime',
        title: 'Agent / Subagent',
        description: '比较主会话和子代理的工具调用质量。',
        height: 560,
        layout: 'compact',
        hasData: data => hasArrayData(data.agent_analysis && data.agent_analysis.agents),
        emptyText: '该时间范围内没有 agent/subagent 数据。',
        render: data => initAgentChart(data.agent_analysis)
    },
    {
        id: 'sessionAnalysisChart',
        group: 'runtime',
        title: 'Session 生命周期',
        description: '跟踪会话耗时、失败和结果状态。',
        height: 620,
        layout: 'wide',
        hasData: data => hasArrayData(data.session_analysis && data.session_analysis.sessions),
        emptyText: '该时间范围内没有 Session 生命周期数据。',
        render: data => initSessionAnalysisChart(data.session_analysis)
    },
    {
        id: 'taskPlanChart',
        group: 'runtime',
        title: 'Task / Plan 结构',
        description: '分析计划模式、任务状态和提醒信号。',
        height: 750,
        layout: 'wide',
        hasData: data => hasTaskPlanData(data.task_plan_analysis),
        emptyText: '该时间范围内没有 Task / Plan 结构数据。',
        render: data => initTaskPlanChart(data.task_plan_analysis)
    }
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
        const originalSetOption = chart.setOption.bind(chart);
        chart.setOption = function(option, ...args) {
            return originalSetOption(normalizeChartOption(option), ...args);
        };
        chartInstances.add(chart);
        return chart;
    };
    window.echarts.__ccInsightsTracked = true;
}

function normalizeChartOption(option) {
    if (!option || typeof option !== 'object') {
        return option;
    }

    if (Object.prototype.hasOwnProperty.call(option, 'title')) {
        option.title = Array.isArray(option.title)
            ? option.title.map(title => ({ ...title, show: false }))
            : { ...option.title, show: false };
    }

    return option;
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
    document.getElementById('summaryRange').textContent = rangeText;

    const totalRecords = (data.commands || []).reduce((sum, cmd) => sum + cmd.count, 0);
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

    renderSummary(data);

    chartGroups.forEach(group => {
        const section = createChartSection(group);
        const grid = section.querySelector('.chart-section-grid');
        chartDefinitions
            .filter(definition => definition.group === group.id)
            .forEach(definition => {
                grid.appendChild(createChartDiv(definition));
            });
        container.appendChild(section);
    });

    container.style.display = '';

    chartDefinitions.forEach(definition => {
        if (definition.hasData && !definition.hasData(data)) {
            renderEmptyChart(definition);
            return;
        }
        definition.render(data);
    });

    normalizeInsightBlocks();
    collapseEmptyCharts();
    updateGroupStates();
    requestAnimationFrame(resizeCharts);
}

function renderSummary(data) {
    const summaryPanel = document.getElementById('section-overview');
    const summaryGrid = document.getElementById('summaryGrid');
    const totalCommands = (data.commands || []).reduce((sum, cmd) => sum + cmd.count, 0);
    const totalMessages = data.project_stats ? data.project_stats.total_messages : (data.daily_trend || { counts: [] }).counts.reduce((sum, count) => sum + count, 0);
    const totalSessions = data.sessions ? data.sessions.total_sessions : 0;
    const totalTools = data.tool_analysis ? data.tool_analysis.total_calls : (data.mcp_tools || []).reduce((sum, tool) => sum + tool.count, 0);
    const failureRate = data.tool_analysis && data.tool_analysis.total_calls > 0
        ? `${((data.tool_analysis.total_failures / data.tool_analysis.total_calls) * 100).toFixed(1)}%`
        : '-';
    const totalTokens = data.cost_analysis && data.cost_analysis.totals
        ? data.cost_analysis.totals.total_tokens
        : (data.model_usage || []).reduce((sum, model) => sum + model.tokens, 0);
    const topProject = data.project_stats && data.project_stats.projects && data.project_stats.projects[0]
        ? shortPath(data.project_stats.projects[0].project)
        : '-';
    const topModel = data.model_usage && data.model_usage[0] ? shortModelName(data.model_usage[0].model) : '-';

    const cards = [
        { label: '消息数', value: formatNumber(totalMessages), meta: `${formatNumber(totalSessions)} 个会话` },
        { label: '命令调用', value: formatNumber(totalCommands), meta: `${(data.commands || []).length} 种命令` },
        { label: '工具调用', value: formatNumber(totalTools), meta: `失败率 ${failureRate}` },
        { label: 'Token', value: compactNumber(totalTokens), meta: `Top 模型 ${topModel}` },
        { label: '活跃项目', value: topProject, meta: data.project_stats ? `${formatNumber(data.project_stats.projects.length)} 个项目` : '-' },
        { label: 'MCP 工具', value: formatNumber((data.mcp_tools || []).length), meta: (data.mcp_tools && data.mcp_tools[0]) ? `${data.mcp_tools[0].server} / ${shortToolName(data.mcp_tools[0].tool)}` : '-' }
    ];

    summaryGrid.replaceChildren(...cards.map(createSummaryCard));
    summaryPanel.style.display = '';
}

function createSummaryCard(card) {
    const item = document.createElement('article');
    item.className = 'summary-card';

    const label = document.createElement('div');
    label.className = 'summary-label';
    label.textContent = card.label;

    const value = document.createElement('div');
    value.className = 'summary-value';
    value.textContent = card.value;

    const meta = document.createElement('div');
    meta.className = 'summary-meta';
    meta.textContent = card.meta;

    item.append(label, value, meta);
    return item;
}

function createChartSection(group) {
    const section = document.createElement('section');
    section.className = 'chart-section';
    section.id = `section-${group.id}`;

    const heading = document.createElement('div');
    heading.className = 'section-heading';

    const title = document.createElement('h2');
    title.textContent = group.title;

    const description = document.createElement('p');
    description.textContent = group.description;

    const grid = document.createElement('div');
    grid.className = 'chart-section-grid';

    heading.append(title, description);
    section.append(heading, grid);
    return section;
}

function hasArrayData(items) {
    return Array.isArray(items) && items.length > 0;
}

function sumBy(items, field) {
    if (!Array.isArray(items)) return 0;
    return items.reduce((sum, item) => sum + Number(item && item[field] ? item[field] : 0), 0);
}

function hasTaskPlanData(taskPlan) {
    if (!taskPlan) return false;
    const lifecycle = taskPlan.plan_lifecycle || {};
    const lifecycleTotal = Number(lifecycle.entry_count || 0) + Number(lifecycle.exit_count || 0) + Number(lifecycle.reentry_count || 0);
    const tasks = taskPlan.tasks || {};
    return lifecycleTotal > 0 ||
        hasArrayData(taskPlan.plan_files) ||
        hasArrayData(tasks.status_distribution) ||
        hasArrayData(tasks.session_task_counts);
}

function renderEmptyChart(definition) {
    const wrapper = document.querySelector(`#${definition.id}`)?.closest('.chart-wrapper');
    if (!wrapper) return;

    const insight = wrapper.querySelector('.chart-insight');
    wrapper.classList.add('empty-chart');
    wrapper.classList.toggle('healthy-empty', definition.state === 'healthy');
    if (insight) {
        insight.textContent = definition.emptyText || '该时间范围内暂无数据。';
    }
}

function updateGroupStates() {
    document.querySelectorAll('.chart-section').forEach(section => {
        const wrappers = Array.from(section.querySelectorAll('.chart-wrapper'));
        const emptyCount = wrappers.filter(wrapper => wrapper.classList.contains('empty-chart')).length;
        section.classList.toggle('mostly-empty', emptyCount > 0 && emptyCount === wrappers.length);
        section.dataset.emptyCount = String(emptyCount);
    });
}

// 创建图表容器
function createChartDiv(definition) {
    const wrapper = document.createElement('div');
    wrapper.className = `chart-wrapper ${definition.layout === 'wide' ? 'wide' : 'compact'}`;

    const header = document.createElement('div');
    header.className = 'chart-card-header';

    const title = document.createElement('h3');
    title.textContent = definition.title || definition.id;

    const description = document.createElement('p');
    description.textContent = definition.description || '';

    const chartDiv = document.createElement('div');
    chartDiv.id = definition.id;
    chartDiv.className = 'chart-canvas';
    chartDiv.style.setProperty('--chart-height', `${definition.height}px`);

    const insightDiv = document.createElement('div');
    insightDiv.id = `${definition.id}-insight`;
    insightDiv.className = 'chart-insight';

    header.append(title, description);
    wrapper.append(header, chartDiv, insightDiv);
    return wrapper;
}

function normalizeInsightBlocks() {
    document.querySelectorAll('.chart-insight').forEach(insight => {
        const text = insight.textContent
            .replace(/^💡\s*/, '')
            .replace(/^数据洞察[:：]\s*/, '')
            .trim();

        if (text) {
            insight.textContent = text;
        }
    });
}

function collapseEmptyCharts() {
    document.querySelectorAll('.chart-wrapper').forEach(wrapper => {
        const chart = wrapper.querySelector('.chart-canvas');
        const insight = wrapper.querySelector('.chart-insight');
        const hasRenderedChart = chart && chart.querySelector('canvas, svg');
        const hasEmptyInsight = insight && insight.textContent.includes('暂无');

        if (hasEmptyInsight && !hasRenderedChart) {
            wrapper.classList.add('empty-chart');
        }
    });
}
