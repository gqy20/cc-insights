function initDashboard() {
    installChartTracking();
    setupChartResizeListener();
    setupEventListeners();
    syncFilterControls();
    loadTimelineIndex().catch(error => {
        console.warn('时间轴加载失败', error);
    });
    loadData();
}

// 初始化
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initDashboard);
} else {
    initDashboard();
}
