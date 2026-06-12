// 当前时间范围
let currentPreset = '30d';

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
