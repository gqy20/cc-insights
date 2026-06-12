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
            left: 'center',
            top: 5
        },
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'cross'
            },
            formatter: function(params) {
                return params.map(item => {
                    const value = item.seriesName === 'Token数'
                        ? formatTokenCount(item.value)
                        : formatNumber(item.value);
                    return `${escapeHtml(item.seriesName)}: ${value}`;
                }).join('<br/>');
            }
        },
        legend: {
            data: ['请求数', 'Token数'],
            top: 60,
            left: 'center'
        },
        grid: {
            top: 100,
            bottom: 70,
            left: 70,
            right: 70,
            containLabel: true
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
                position: 'right',
                axisLabel: {
                    formatter: value => formatTokenCount(value)
                }
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
        `<strong>💡 数据洞察:</strong> 最常用的是 <strong>${escapeHtml(topModel.model)}</strong>（${topModel.count.toLocaleString()} 次请求，占比 ${topModelShare}%），` +
        `总计 <strong>${totalRequests.toLocaleString()}</strong> 次请求，` +
        `消耗 <strong>${formatTokenCount(totalTokens)}</strong> Tokens，` +
        `平均每次请求 <strong>${formatTokenCount(avgTokensPerRequest)}</strong> Tokens。`;
}

// 初始化工作时段热力图
function initWorkHoursChart(workHoursData) {
    if (!workHoursData || !workHoursData.hourly_data || workHoursData.hourly_data.length === 0) {
        document.getElementById('workHoursChart-insight').innerHTML =
            '<strong>💡 数据洞察:</strong> 该时间范围内暂无工作时段数据';
        return;
    }

    const chart = echarts.init(document.getElementById('workHoursChart'), 'wonderland');

    const hours = workHoursData.hourly_data;
    const hourLabels = hours.map(h => h.hour_label);
    const counts = hours.map(h => h.count);

    // 工作时段高亮色
    const colors = hours.map(h => {
        if (h.is_work_hour) {
            return '#5470c6';
        }
        return '#bdc3c7';
    });

    const option = {
        title: {
            text: '工作时段热力图',
            subtext: '数据来源: projects/*.jsonl',
            left: 'center'
        },
        tooltip: {
            trigger: 'axis',
            formatter: function(params) {
                const idx = params[0].dataIndex;
                const item = hours[idx];
                const timeType = item.is_work_hour ? '工作时间' : '非工作时间';
                return `${item.hour_label}<br/>` +
                       `${timeType}<br/>` +
                       `活动次数: ${item.count.toLocaleString()}`;
            }
        },
        xAxis: {
            type: 'category',
            data: hourLabels,
            axisLabel: {
                interval: 2,
                rotate: 0
            }
        },
        yAxis: {
            type: 'value',
            name: '活动次数'
        },
        visualMap: {
            show: false,
            dimension: 0,
            pieces: [
                {lte: 8, color: '#bdc3c7'},
                {gt: 8, lte: 18, color: '#5470c6'},
                {gt: 18, color: '#bdc3c7'}
            ]
        },
        series: [{
            name: '活动次数',
            type: 'bar',
            data: counts.map((count, idx) => ({
                value: count,
                itemStyle: { color: colors[idx] }
            })),
            label: {
                show: true,
                position: 'top',
                formatter: function(v) {
                    return v.data.value.toLocaleString();
                }
            },
            markArea: {
                silent: true,
                itemStyle: {
                    color: 'rgba(84, 112, 198, 0.1)'
                },
                data: [[{
                    name: '工作时段',
                    xAxis: '09:00'
                }, {
                    xAxis: '18:00'
                }]]
            }
        }]
    };

    chart.setOption(option);

    // 生成数据洞察
    const peakHour = workHoursData.peak_hour;
    const peakCount = workHoursData.peak_count.toLocaleString();
    const workRatio = workHoursData.work_ratio.toFixed(1);
    const workCount = workHoursData.work_hours.toLocaleString();
    const offCount = workHoursData.off_hours.toLocaleString();

    document.getElementById('workHoursChart-insight').innerHTML =
        `<strong>💡 数据洞察:</strong> 峰值在 <strong>${peakHour}:00</strong>（${peakCount} 次），` +
        `工作时段(9-18点)占比 <strong>${workRatio}%</strong>（${workCount} 次），` +
        `非工作时段 <strong>${offCount}</strong> 次。` +
        (workRatio > 60 ? ' 主要在工作时段活动。' : workRatio < 40 ? ' 经常在非工作时间工作。' : '');
}
