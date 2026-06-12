function initDashboard() {
    installChartTracking();
    setupChartResizeListener();
    setupEventListeners();
    loadData('preset=30d');
}

// 初始化
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initDashboard);
} else {
    initDashboard();
}
