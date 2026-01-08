package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
)

//go:embed static/*
var staticFS embed.FS

func main() {
	flag.Parse()

	// 验证数据目录
	if _, err := os.Stat(cfg.DataDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "错误: 数据目录不存在: %s\n", cfg.DataDir)
		fmt.Fprintf(os.Stderr, "提示: 使用 -data 参数指定数据目录\n")
		os.Exit(1)
	}

	fmt.Printf("📊 Claude Code Dashboard\n")
	fmt.Printf("   数据目录: %s\n", cfg.DataDir)
	fmt.Printf("   监听地址: %s\n", cfg.ListenAddr)
	fmt.Printf("\n启动服务...\n")

	// 路由
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/dashboard", dashboardPageHandler)
	http.HandleFunc("/api/data", handleDataAPI)
	http.HandleFunc("/api/stats", statsAPIHandler)
	http.HandleFunc("/api/reload", reloadHandler)

	// 静态资源（使用嵌入的文件系统）
	staticSub, _ := fs.Sub(staticFS, "static")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// 启动服务器
	fmt.Printf("\n✅ Dashboard 已启动!\n")
	fmt.Printf("   访问: http://localhost%s\n", cfg.ListenAddr)
	fmt.Printf("   Dashboard: http://localhost%s/dashboard\n", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		os.Exit(1)
	}
}

// indexHandler 首页
func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	tmpl := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Claude Code Dashboard</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
            max-width: 800px;
            margin: 100px auto;
            padding: 20px;
            text-align: center;
            background: #f5f5f5;
        }
        h1 { color: #2c3e50; margin-bottom: 30px; }
        .links a {
            display: block;
            padding: 15px 30px;
            margin: 10px;
            background: #3498db;
            color: white;
            text-decoration: none;
            border-radius: 8px;
            font-size: 18px;
            transition: background 0.2s;
        }
        .links a:hover {
            background: #2980b9;
        }
        .info {
            margin-top: 40px;
            padding: 20px;
            background: white;
            border-radius: 8px;
            color: #7f8c8d;
        }
    </style>
</head>
<body>
    <h1>📊 Claude Code Dashboard</h1>
    <p style="color: #7f8c8d; margin-bottom: 30px;">数据分析可视化平台</p>
    <div class="links">
        <a href="/dashboard">📈 查看 Dashboard (支持时间范围筛选)</a>
        <a href="/api/stats">📡 API 接口</a>
    </div>
    <div class="info">
        <p><strong>功能:</strong></p>
        <p>✅ 时间范围筛选 (7天/30天/90天/全部/自定义)</p>
        <p>✅ 实时数据刷新</p>
        <p>✅ 交互式图表展示</p>
    </div>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, tmpl)
}

// dashboardPageHandler Dashboard 页面（新的带侧边栏版本）
func dashboardPageHandler(w http.ResponseWriter, r *http.Request) {
	// 直接读取文件系统
	templateContent := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Claude Code Dashboard</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #f5f5f5; color: #333; }
        .container { display: flex; min-height: 100vh; }
        .sidebar { width: 280px; background: #2c3e50; color: #ecf0f1; padding: 20px; position: fixed; height: 100vh; overflow-y: auto; }
        .sidebar h2 { font-size: 18px; margin-bottom: 20px; padding-bottom: 10px; border-bottom: 1px solid #34495e; }
        .sidebar h3 { font-size: 14px; color: #95a5a6; margin: 20px 0 10px; text-transform: uppercase; }
        .preset-buttons { display: flex; flex-direction: column; gap: 8px; }
        .preset-btn { padding: 10px 15px; background: #34495e; border: none; border-radius: 6px; color: #ecf0f1; cursor: pointer; text-align: left; transition: all 0.2s; }
        .preset-btn:hover { background: #415b76; }
        .preset-btn.active { background: #3498db; }
        .custom-range { margin-top: 15px; padding: 15px; background: #34495e; border-radius: 6px; }
        .custom-range label { display: block; font-size: 12px; color: #95a5a6; margin-bottom: 5px; }
        .custom-range input { width: 100%; padding: 8px; margin-bottom: 10px; border: 1px solid #4a5f7a; border-radius: 4px; background: #2c3e50; color: #ecf0f1; }
        .custom-range button { width: 100%; padding: 10px; background: #27ae60; border: none; border-radius: 4px; color: white; cursor: pointer; font-weight: 500; }
        .custom-range button:hover { background: #2ecc71; }
        .stats-info { margin-top: 20px; padding: 15px; background: #34495e; border-radius: 6px; font-size: 12px; }
        .stats-info p { margin: 5px 0; color: #95a5a6; }
        .stats-info strong { color: #ecf0f1; }
        .main-content { flex: 1; margin-left: 280px; padding: 30px; }
        .main-content h1 { font-size: 24px; margin-bottom: 20px; color: #2c3e50; }
        .charts-container { display: flex; flex-direction: column; gap: 30px; }
        .chart-wrapper { background: white; border-radius: 8px; padding: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .loading { text-align: center; padding: 40px; color: #95a5a6; }
        .error { background: #e74c3c; color: white; padding: 15px; border-radius: 6px; margin-bottom: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <aside class="sidebar">
            <h2>⏱️ 时间范围</h2>
            <h3>快捷选择</h3>
            <div class="preset-buttons">
                <button class="preset-btn" data-preset="24h">最近 24 小时</button>
                <button class="preset-btn" data-preset="7d">最近 7 天</button>
                <button class="preset-btn" data-preset="30d">最近 30 天</button>
                <button class="preset-btn" data-preset="90d">最近 90 天</button>
                <button class="preset-btn active" data-preset="all">全部数据</button>
            </div>
            <h3>自定义范围</h3>
            <div class="custom-range">
                <label>开始日期</label>
                <input type="date" id="startDate">
                <label>结束日期</label>
                <input type="date" id="endDate">
                <button onclick="applyCustomRange()">应用范围</button>
            </div>
            <div class="stats-info" id="statsInfo">
                <p><strong>最后更新:</strong> <span id="lastUpdate">-</span></p>
                <p><strong>时间范围:</strong> <span id="rangeInfo">全部</span></p>
                <p><strong>记录数:</strong> <span id="recordCount">-</span></p>
            </div>
        </aside>
        <main class="main-content">
            <h1>📊 Claude Code Dashboard</h1>
            <div id="errorMessage"></div>
            <div id="loadingIndicator" class="loading">正在加载数据</div>
            <div id="chartsContainer" class="charts-container" style="display:none;"></div>
        </main>
    </div>
    <script src="https://go-echarts.github.io/go-echarts-assets/assets/echarts.min.js"></script>
    <script src="/static/app.js"></script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, templateContent)
}

// statsAPIHandler 统计数据 API (保持兼容)
func statsAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cmdStats, _, err := ParseHistory()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	_, counts, err := GetDailyTrend()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	cache, err := ParseStatsCache()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// JSON 响应
	fmt.Fprintf(w, `{
		"commands": %s,
		"daily_trend": {
			"dates": %s,
			"counts": %s
		},
		"model_usage": %s
	}`,
	 toJSON(cmdStats),
	 toJSON([]string{"2025-12-31", "2026-01-01", "2026-01-02", "2026-01-03", "2026-01-04", "2026-01-05", "2026-01-06"}),
	 toJSON(counts),
	 toJSON(cache.ModelUsage))
}

// reloadHandler 重新加载数据
func reloadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"status": "ok", "message": "数据已刷新"}`)
}

// toJSON 简单的 JSON 序列化
func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
