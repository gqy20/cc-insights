// 全局工作台状态。所有图表、诊断和下钻请求都从同一组 filters 生成。
let currentPreset = '30d';
let dashboardAbortController = null;
let filterDebounceTimer = null;

const dashboardState = {
    filters: {
        preset: '30d',
        start: '',
        end: '',
        project: '',
        tool: '',
        model: '',
        reason: '',
        limit: 12,
        samples: 8,
        detail: true
    },
    data: {},
    timelineDays: [],
    selectedDiagnosticID: ''
};

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
    "正在读取聚合缓存...",
    "正在刷新交互式 API...",
    "正在同步诊断和下钻数据...",
    "正在更新图表...",
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
