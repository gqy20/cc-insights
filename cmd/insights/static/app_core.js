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
        id: 'runtimeTools',
        group: 'runtime',
        title: 'Runtime 工具信号',
        description: '从 debug 日志补充 MCP runtime 工具信号，不代表完整工具调用口径。',
        height: 700,
        layout: 'compact',
        hasData: data => hasArrayData(data.runtime_tools),
        emptyText: '该时间范围内没有 Runtime 工具信号。',
        render: data => initRuntimeToolsChart(data.runtime_tools)
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
        id: 'skillAnalysisChart',
        group: 'runtime',
        title: 'Skills',
        description: '统计本地安装、可见性、调用结果和 session 关联工具。',
        height: 620,
        layout: 'wide',
        hasData: data => hasArrayData(data.skill_analysis && data.skill_analysis.skills) || hasArrayData(data.skill_analysis && data.skill_analysis.installed),
        emptyText: '该时间范围内没有 skill 调用或安装数据。',
        render: data => initSkillAnalysisChart(data.skill_analysis)
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

    normalizeAxes(option.xAxis);
    normalizeAxes(option.yAxis);

    return option;
}

function normalizeAxes(axes) {
    if (!axes) return;
    const axisList = Array.isArray(axes) ? axes : [axes];
    axisList.forEach(axis => {
        if (!axis || typeof axis !== 'object') return;
        axis.splitLine = { ...(axis.splitLine || {}), show: false };
    });
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

function setupEventListeners() {
    document.querySelectorAll('.preset-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            setFilter({ preset: btn.dataset.preset, start: '', end: '' }, { immediate: true });
        });
    });

    const today = new Date().toISOString().split('T')[0];
    const weekAgo = dateOffset(today, -6);
    setInputValue('endDate', today);
    setInputValue('startDate', weekAgo);

    bindClick('applyRangeBtn', applyCustomRange);
    bindClick('clearFiltersBtn', clearSecondaryFilters);
    bindInputFilter('projectFilter', 'project');
    bindInputFilter('toolFilter', 'tool');
    bindInputFilter('modelFilter', 'model');
    bindInputFilter('reasonFilter', 'reason');

    const slider = document.getElementById('timelineSlider');
    if (slider) {
        slider.addEventListener('input', () => applyTimelineWindow(Number(slider.value)));
    }
    const windowSize = document.getElementById('windowSize');
    if (windowSize) {
        windowSize.addEventListener('change', () => {
            const currentIndex = Number(document.getElementById('timelineSlider')?.value || 0);
            applyTimelineWindow(currentIndex);
        });
    }
}

function bindClick(id, handler) {
    const el = document.getElementById(id);
    if (el) el.addEventListener('click', handler);
}

function bindInputFilter(id, key) {
    const el = document.getElementById(id);
    if (!el) return;
    el.addEventListener('input', () => {
        setFilter({ [key]: el.value.trim() }, { immediate: false });
    });
}

function setFilter(next, options = {}) {
    dashboardState.filters = { ...dashboardState.filters, ...next };
    currentPreset = dashboardState.filters.preset || 'custom';
    syncFilterControls();
    scheduleLoad(Boolean(options.immediate));
}

function setActivePreset(preset) {
    currentPreset = preset;
    dashboardState.filters.preset = preset;
    syncFilterControls();
}

function syncFilterControls() {
    document.querySelectorAll('.preset-btn').forEach(btn => {
        btn.classList.toggle('active', dashboardState.filters.preset === btn.dataset.preset && !dashboardState.filters.start && !dashboardState.filters.end);
    });
    setInputValue('startDate', dashboardState.filters.start);
    setInputValue('endDate', dashboardState.filters.end);
    setInputValue('projectFilter', dashboardState.filters.project);
    setInputValue('toolFilter', dashboardState.filters.tool);
    setInputValue('modelFilter', dashboardState.filters.model);
    setInputValue('reasonFilter', dashboardState.filters.reason);
}

function setInputValue(id, value) {
    const input = document.getElementById(id);
    if (input && input.value !== value) input.value = value || '';
}

function applyCustomRange() {
    const startDate = document.getElementById('startDate')?.value || '';
    const endDate = document.getElementById('endDate')?.value || '';
    if (!startDate || !endDate) {
        showError('请选择开始和结束日期');
        return;
    }
    if (new Date(startDate) > new Date(endDate)) {
        showError('开始日期不能晚于结束日期');
        return;
    }
    setFilter({ preset: 'custom', start: startDate, end: endDate }, { immediate: true });
}

function clearSecondaryFilters() {
    dashboardState.selectedDiagnosticID = '';
    setFilter({ project: '', tool: '', model: '', reason: '' }, { immediate: true });
}

function scheduleLoad(immediate) {
    clearTimeout(filterDebounceTimer);
    filterDebounceTimer = setTimeout(() => loadData(), immediate ? 0 : 350);
}

async function loadTimelineIndex() {
    const result = await fetchInteractive('/api/timeline', { preset: 'all', limit: 5000 }, null);
    dashboardState.timelineDays = (result.data && result.data.days) || [];
    renderTimelineControl();
}

async function loadData(params) {
    if (params) {
        const parsed = Object.fromEntries(new URLSearchParams(params).entries());
        dashboardState.filters = { ...dashboardState.filters, ...parsed };
    }
    const preset = dashboardState.filters.preset || '30d';
    showLoading(true, preset);
    hideError();

    if (dashboardAbortController) {
        dashboardAbortController.abort();
    }
    dashboardAbortController = new AbortController();
    const signal = dashboardAbortController.signal;

    let stageIndex = 0;
    const stageInterval = setInterval(() => {
        if (stageIndex < loadingStages.length) {
            updateLoadingProgress(loadingStages[stageIndex]);
            stageIndex++;
        }
    }, 500);

    try {
        const filterParams = buildQueryParams();
        const [dashboard, overview, diagnostics, failures, commands, tokens, sessions, tools] = await Promise.all([
            fetchDashboardData(filterParams, signal),
            fetchInteractive('/api/overview', filterParams, signal),
            fetchInteractive('/api/diagnostics', { ...filterParams, detail: 'true' }, signal),
            fetchInteractive('/api/detail/failures', filterParams, signal),
            fetchInteractive('/api/detail/commands', filterParams, signal),
            fetchInteractive('/api/detail/tokens', filterParams, signal),
            fetchInteractive('/api/detail/sessions', filterParams, signal),
            fetchInteractive('/api/detail/tools', filterParams, signal)
        ]);

        clearInterval(stageInterval);
        updateLoadingProgress(loadingStages[loadingStages.length - 1]);

        dashboardState.data = { dashboard, overview, diagnostics, failures, commands, tokens, sessions, tools };
        updateStatsInfo(dashboard.data, overview.meta);
        renderSummary(dashboard.data, overview.data);
        renderDiagnostics(diagnostics.data);
        renderDetails({ failures: failures.data, commands: commands.data, tokens: tokens.data, sessions: sessions.data, tools: tools.data });
        renderCharts(dashboard.data, { skipSummary: true });
    } catch (error) {
        clearInterval(stageInterval);
        if (error.name !== 'AbortError') {
            showError('加载数据失败: ' + error.message);
        }
    } finally {
        if (!signal.aborted) {
            showLoading(false);
        }
    }
}

function buildQueryParams() {
    const params = {};
    const filter = dashboardState.filters;
    const add = (key, value) => {
        if (value !== undefined && value !== null && String(value).trim() !== '') {
            params[key] = String(value).trim();
        }
    };
    add('preset', filter.preset || '30d');
    add('start', filter.start);
    add('end', filter.end);
    add('limit', filter.limit);
    add('samples', filter.samples);
    add('project', filter.project);
    add('tool', filter.tool);
    add('model', filter.model);
    add('reason', filter.reason);
    if (filter.detail) add('detail', 'true');
    if (dashboardState.selectedDiagnosticID) add('id', dashboardState.selectedDiagnosticID);
    return params;
}

function toQueryString(params) {
    return new URLSearchParams(params).toString();
}

async function fetchDashboardData(params, signal) {
    const response = await fetch(`/api/data?${toQueryString(params)}`, { signal });
    const result = await response.json();
    if (!result.success) throw new Error(result.error || 'Dashboard 数据接口返回失败');
    return result;
}

async function fetchInteractive(endpoint, params, signal) {
    const response = await fetch(`${endpoint}?${toQueryString(params)}`, signal ? { signal } : undefined);
    const result = await response.json();
    if (!result.success) throw new Error(result.error || `${endpoint} 返回失败`);
    return result;
}

function renderTimelineControl() {
    const slider = document.getElementById('timelineSlider');
    const label = document.getElementById('timelineLabel');
    const days = dashboardState.timelineDays;
    if (!slider || !label) return;
    if (!days.length) {
        slider.disabled = true;
        label.textContent = '没有可滑动的历史日期';
        return;
    }
    slider.disabled = false;
    slider.min = '0';
    slider.max = String(days.length - 1);
    slider.value = String(days.length - 1);
    label.textContent = `${days[0].date} 至 ${days[days.length - 1].date}`;
}

function applyTimelineWindow(index) {
    const days = dashboardState.timelineDays;
    if (!days.length || !days[index]) return;
    const size = Number(document.getElementById('windowSize')?.value || 7);
    const end = days[index].date;
    const startIndex = Math.max(0, index - size + 1);
    const start = days[startIndex].date;
    const label = document.getElementById('timelineLabel');
    if (label) {
        const day = days[index];
        label.textContent = `${start} 至 ${end}，消息 ${formatNumber(day.messages)}，Token ${compactNumber(day.tokens)}`;
    }
    setFilter({ preset: 'custom', start, end }, { immediate: false });
}

function dateOffset(dateString, offset) {
    const date = new Date(`${dateString}T00:00:00`);
    date.setDate(date.getDate() + offset);
    return date.toISOString().split('T')[0];
}

// 更新加载进度文本
function updateLoadingProgress(text) {
    const progressEl = document.getElementById('loadingProgress');
    if (progressEl) {
        progressEl.textContent = text;
    }
}

// 更新统计信息
function updateStatsInfo(data, meta) {
    document.getElementById('lastUpdate').textContent = data.timestamp || (meta && meta.generated_at) || '-';
    const sourceEl = document.getElementById('dataSource');
    if (sourceEl) sourceEl.textContent = meta && meta.source ? meta.source : '-';
    const runtimeEl = document.getElementById('runtimeInfo');
    if (runtimeEl) runtimeEl.textContent = meta && Number.isFinite(meta.runtime_ms) ? `${meta.runtime_ms} ms` : '-';

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
    document.getElementById('summaryRange').textContent = rangeText;

    const totalRecords = safeArray(data.commands).reduce((sum, cmd) => sum + cmd.count, 0);
    const context = document.getElementById('detailContext');
    if (context) context.textContent = `${rangeText}，Slash 命令 ${totalRecords.toLocaleString()} 次。`;
}

// 渲染图表
function renderCharts(data, options = {}) {
    const container = document.getElementById('chartsContainer');
    disposeCharts();
    container.replaceChildren();

    if (!options.skipSummary) {
        renderSummary(data);
    }

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
        const coverage = getChartCoverage(data, definition.id);
        setChartCoverageState(definition, coverage);
        if (coverage.status === 'unavailable') {
            renderEmptyChart(definition, coverage.reason || '当前筛选条件下该图没有精确数据。', coverage);
            return;
        }
        if (definition.hasData && !definition.hasData(data)) {
            renderEmptyChart(definition, definition.emptyText, coverage);
            return;
        }
        definition.render(data);
        annotateChartCoverage(definition, coverage);
    });

    normalizeInsightBlocks();
    collapseEmptyCharts();
    updateGroupStates();
    requestAnimationFrame(resizeCharts);
}

function renderSummary(data, overview) {
    const summaryPanel = document.getElementById('section-overview');
    const summaryGrid = document.getElementById('summaryGrid');
    const summary = overview && overview.summary ? overview.summary : {};
    const top = overview && overview.top ? overview.top : {};
    const totalCommands = summary.commands || safeArray(data.commands).reduce((sum, cmd) => sum + cmd.count, 0);
    const dailyCounts = safeArray(data.daily_trend && data.daily_trend.counts);
    const totalMessages = summary.messages || (data.project_stats ? data.project_stats.total_messages : dailyCounts.reduce((sum, count) => sum + count, 0));
    const totalSessions = summary.sessions || (data.sessions ? data.sessions.total_sessions : 0);
    const totalTools = summary.tool_calls || (data.tool_analysis ? data.tool_analysis.total_calls : 0);
    const failureRate = Number(summary.failure_rate) > 0
        ? `${summary.failure_rate.toFixed(1)}%`
        : data.tool_analysis && data.tool_analysis.total_calls > 0
        ? `${((data.tool_analysis.total_failures / data.tool_analysis.total_calls) * 100).toFixed(1)}%`
        : '-';
    const totalTokens = summary.tokens || (data.cost_analysis && data.cost_analysis.totals
        ? data.cost_analysis.totals.total_tokens
        : safeArray(data.model_usage).reduce((sum, model) => sum + model.tokens, 0));
    const topProject = top.projects && top.projects[0]
        ? shortPath(top.projects[0].project)
        : data.project_stats && data.project_stats.projects && data.project_stats.projects[0]
        ? shortPath(data.project_stats.projects[0].project)
        : '-';
    const topModel = top.models && top.models[0]
        ? shortModelName(top.models[0].model)
        : safeArray(data.model_usage)[0] ? shortModelName(safeArray(data.model_usage)[0].model) : '-';
    const primaryMetric = buildPrimarySummaryMetric(data, summary, {
        messages: totalMessages,
        sessions: totalSessions,
        tools: totalTools,
        commands: totalCommands
    });

    const cards = [
        primaryMetric,
        { label: '命令调用', value: formatNumber(totalCommands), meta: `${safeArray(data.commands).length} 种命令` },
        { label: '工具调用', value: formatNumber(totalTools), meta: `失败率 ${failureRate}` },
        { label: 'Token', value: compactNumber(totalTokens), meta: `Top 模型 ${topModel}` },
        { label: '活跃项目', value: topProject, meta: `${formatNumber(summary.projects || (data.project_stats ? data.project_stats.projects.length : 0))} 个项目` },
        { label: '诊断项', value: formatNumber((overview && overview.diagnostics && overview.diagnostics.total) || 0), meta: '可点击查看证据' }
    ];

    summaryGrid.replaceChildren(...cards.map(createSummaryCard));
    summaryPanel.style.display = '';
}

function buildPrimarySummaryMetric(data, summary, totals) {
    const filters = dashboardState.filters || {};
    const dailyTotal = safeArray(data.daily_trend && data.daily_trend.counts).reduce((sum, count) => sum + count, 0);
    if (filters.tool) {
        if (data.coverage && data.coverage.dailyTrend && data.coverage.dailyTrend.status === 'unavailable') {
            return { label: '筛选命中', value: '-', meta: '当前组合暂不支持精确计算' };
        }
        return { label: '筛选命中', value: formatNumber(totals.tools || dailyTotal), meta: `工具 ${filters.tool}` };
    }
    if (filters.reason) {
        if (data.coverage && data.coverage.dailyTrend && data.coverage.dailyTrend.status === 'unavailable') {
            return { label: '筛选命中', value: '-', meta: '当前组合暂不支持精确计算' };
        }
        const failures = summary.failures || (data.failure_analysis ? data.failure_analysis.total_failures : dailyTotal);
        return { label: '筛选命中', value: formatNumber(failures), meta: `失败原因 ${filters.reason}` };
    }
    if (filters.model) {
        return { label: '筛选命中', value: formatNumber(totals.messages || dailyTotal), meta: `模型 ${shortModelName(filters.model)}` };
    }
    if (filters.project) {
        return { label: '筛选命中', value: formatNumber(totals.messages || dailyTotal), meta: shortPath(filters.project) };
    }
    if (filters.family) {
        return { label: '筛选命中', value: formatNumber(totals.commands || dailyTotal), meta: `命令族 ${filters.family}` };
    }
    return { label: '消息数', value: formatNumber(totals.messages), meta: `${formatNumber(totals.sessions)} 个会话` };
}

function renderDiagnostics(report) {
    const panel = document.getElementById('section-diagnostics');
    const list = document.getElementById('diagnosticList');
    if (!panel || !list) return;

    const items = report && Array.isArray(report.recommendations) ? report.recommendations : [];
    if (items.length === 0) {
        list.replaceChildren(createEmptyBlock('当前过滤条件下没有触发诊断建议。'));
        panel.style.display = '';
        return;
    }

    list.replaceChildren(...items.map(item => {
        const card = document.createElement('button');
        card.type = 'button';
        card.className = 'diagnostic-card';
        card.classList.toggle('active', dashboardState.selectedDiagnosticID === item.id);
        card.addEventListener('click', () => {
            dashboardState.selectedDiagnosticID = dashboardState.selectedDiagnosticID === item.id ? '' : item.id;
            loadData();
        });

        const head = document.createElement('div');
        head.className = 'diagnostic-card-head';
        const title = document.createElement('strong');
        title.textContent = item.title || item.id || '未命名诊断';
        const badge = document.createElement('span');
        badge.className = `severity-badge severity-${(item.severity || 'info').toLowerCase()}`;
        badge.textContent = item.severity || 'info';
        head.append(title, badge);

        const desc = document.createElement('p');
        desc.textContent = item.summary || item.description || '暂无说明';

        const evidence = document.createElement('div');
        evidence.className = 'diagnostic-evidence';
        (item.evidence || []).slice(0, 3).forEach(ev => {
            const chip = document.createElement('span');
            chip.textContent = `${ev.label || ev.name || '证据'}: ${ev.value || '-'}`;
            evidence.appendChild(chip);
        });

        card.append(head, desc, evidence);
        return card;
    }));
    panel.style.display = '';
}

function renderDetails(details) {
    const panel = document.getElementById('section-details');
    const grid = document.getElementById('detailGrid');
    if (!panel || !grid) return;

    const blocks = [
        createFailureDetail(details.failures),
        createCommandDetail(details.commands),
        createTokenDetail(details.tokens),
        createSessionDetail(details.sessions),
        createToolDetail(details.tools)
    ].filter(Boolean);

    grid.replaceChildren(...blocks);
    panel.style.display = '';
}

function createFailureDetail(report) {
    const summary = report && report.summary ? report.summary : {};
    const samples = Array.isArray(report && report.samples) ? report.samples : [];
    const items = [
        metricLine('匹配样例', formatNumber(summary.matched_samples || samples.length)),
        metricLine('可用样例', formatNumber(summary.available_samples || 0)),
        listLine('主要原因', (summary.top_reasons || []).map(item => `${item.name} (${item.count})`))
    ];
    return createDetailCard('失败来源', 'reason 过滤会直接作用到失败样例。', items, samples.slice(0, 3).map(sample => sample.error || sample.message || sample.reason || sample.tool).filter(Boolean));
}

function createCommandDetail(report) {
    const families = Array.isArray(report && report.by_family) ? report.by_family : [];
    const risky = Array.isArray(report && report.risky_commands) ? report.risky_commands : [];
    const items = [
        metricLine('Bash 调用', formatNumber(report && report.total_calls)),
        metricLine('命令数量', formatNumber(report && report.total_commands)),
        listLine('高频命令族', families.slice(0, 3).map(item => `${item.family} (${item.call_count})`))
    ];
    return createDetailCard('Bash 命令', '用于判断是否需要封装 hooks 或稳定工具。', items, risky.slice(0, 3).map(item => `${item.command_name || item.command}: ${item.risk_level || '-'}`));
}

function createTokenDetail(report) {
    const projects = Array.isArray(report && report.by_project) ? report.by_project : [];
    const sessions = Array.isArray(report && report.by_session) ? report.by_session : [];
    const models = Array.isArray(report && report.by_model) ? report.by_model : [];
    const items = [
        listLine('模型消耗', models.slice(0, 3).map(item => `${shortModelName(item.model)} ${compactNumber(item.total_tokens || item.tokens)}`)),
        listLine('项目消耗', projects.slice(0, 3).map(item => `${shortPath(item.project)} ${compactNumber(item.total_tokens || item.tokens)}`))
    ];
    return createDetailCard('Token 与成本', '优先定位大头项目、模型和 session。', items, sessions.slice(0, 3).map(item => `${shortAgentID(item.session_id)} ${compactNumber(item.total_tokens || item.tokens)}`));
}

function createSessionDetail(report) {
    const longRunning = Array.isArray(report && report.long_running) ? report.long_running : [];
    const failures = Array.isArray(report && report.top_failures) ? report.top_failures : [];
    const items = [
        metricLine('匹配 Session', formatNumber(report && report.total_sessions)),
        listLine('长会话', longRunning.slice(0, 3).map(item => `${shortAgentID(item.session_id)} ${formatDuration(item.duration_ms || 0)}`)),
        listLine('失败会话', failures.slice(0, 3).map(item => `${shortAgentID(item.session_id)} ${item.tool_failure_count || 0} 次`))
    ];
    return createDetailCard('Session 生命周期', '用于观察任务拆分、长会话和失败集中度。', items, report && report.insights ? report.insights.slice(0, 3) : []);
}

function createToolDetail(report) {
    const categories = Array.isArray(report && report.by_category) ? report.by_category : [];
    const slowest = Array.isArray(report && report.slowest_calls) ? report.slowest_calls : [];
    const items = [
        listLine('工具耗时', categories.slice(0, 3).map(item => `${item.category || item.base_tool} ${formatDuration(item.total_duration_ms || 0)}`)),
        listLine('最慢调用', slowest.slice(0, 3).map(item => `${item.category || item.tool} ${formatDuration(item.duration_ms || 0)}`))
    ];
    return createDetailCard('工具性能', '用于定位慢工具、错误率和缺失结果。', items, report && report.insights ? report.insights.slice(0, 3) : []);
}

function createDetailCard(title, description, metrics, notes) {
    const card = document.createElement('article');
    card.className = 'detail-card';
    const h3 = document.createElement('h3');
    h3.textContent = title;
    const desc = document.createElement('p');
    desc.textContent = description;
    const body = document.createElement('div');
    body.className = 'detail-lines';
    metrics.forEach(line => body.appendChild(line));
    const noteList = document.createElement('ul');
    noteList.className = 'detail-notes';
    (notes || []).forEach(note => {
        const li = document.createElement('li');
        li.textContent = note;
        noteList.appendChild(li);
    });
    card.append(h3, desc, body);
    if (noteList.children.length > 0) card.appendChild(noteList);
    return card;
}

function metricLine(label, value) {
    const row = document.createElement('div');
    row.className = 'detail-line';
    row.innerHTML = `<span>${escapeHtml(label)}</span><strong>${escapeHtml(value)}</strong>`;
    return row;
}

function listLine(label, values) {
    return metricLine(label, values && values.length ? values.join(' / ') : '-');
}

function createEmptyBlock(text) {
    const block = document.createElement('div');
    block.className = 'empty-block';
    block.textContent = text;
    return block;
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

function safeArray(items) {
    return Array.isArray(items) ? items : [];
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

function renderEmptyChart(definition, message, coverage = {}) {
    const wrapper = document.querySelector(`#${definition.id}`)?.closest('.chart-wrapper');
    if (!wrapper) return;

    const insight = wrapper.querySelector('.chart-insight');
    wrapper.classList.add('empty-chart');
    wrapper.classList.toggle('coverage-unavailable', coverage.status === 'unavailable');
    wrapper.classList.toggle('healthy-empty', definition.state === 'healthy');
    if (insight) {
        insight.textContent = message || definition.emptyText || '该时间范围内暂无数据。';
    }
}

function getChartCoverage(data, chartID) {
    const coverage = data && data.coverage && data.coverage[chartID];
    return coverage || { status: 'exact' };
}

function setChartCoverageState(definition, coverage) {
    const wrapper = document.querySelector(`#${definition.id}`)?.closest('.chart-wrapper');
    if (!wrapper) return;
    wrapper.classList.toggle('coverage-sample', coverage.status === 'sample');
    wrapper.classList.toggle('coverage-unavailable', coverage.status === 'unavailable');

    const badge = wrapper.querySelector('.coverage-badge');
    if (!badge) return;
    if (!coverage.status || coverage.status === 'exact') {
        badge.hidden = true;
        badge.textContent = '';
        return;
    }
    badge.hidden = false;
    badge.textContent = coverage.status === 'sample' ? '样本' : '不可用';
    badge.title = coverage.reason || '';
}

function annotateChartCoverage(definition, coverage) {
    if (!coverage || !coverage.status || coverage.status === 'exact' || coverage.status === 'unavailable') return;
    const wrapper = document.querySelector(`#${definition.id}`)?.closest('.chart-wrapper');
    const insight = wrapper && wrapper.querySelector('.chart-insight');
    if (insight && coverage.reason && !insight.textContent.trim()) {
        insight.textContent = coverage.reason;
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

    const titleRow = document.createElement('div');
    titleRow.className = 'chart-title-row';
    const badge = document.createElement('span');
    badge.className = 'coverage-badge';
    badge.hidden = true;
    titleRow.append(title, badge);

    const description = document.createElement('p');
    description.textContent = definition.description || '';

    const chartDiv = document.createElement('div');
    chartDiv.id = definition.id;
    chartDiv.className = 'chart-canvas';
    chartDiv.style.setProperty('--chart-height', `${definition.height}px`);

    const insightDiv = document.createElement('div');
    insightDiv.id = `${definition.id}-insight`;
    insightDiv.className = 'chart-insight';

    header.append(titleRow, description);
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
