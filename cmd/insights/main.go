//go:build !bench

package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

//go:embed static/*
var staticFS embed.FS

// 版本信息（通过 -ldflags 注入）
var version = "dev"
var commit = "unknown"
var buildDate = ""

func main() {
	if err := runCLI(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runWebServer() error {
	// 初始化日志系统（在所有操作之前）
	logDir := filepath.Join(filepath.Dir(cfg.CacheDir), "logs")
	if err := InitLogger(logDir); err != nil {
		fmt.Fprintf(os.Stderr, "日志初始化失败: %v\n", err)
	}
	defer CloseLogger()

	Info("Claude Code Dashboard 启动",
		"version", version,
		"commit", commit,
	)

	// 验证数据目录
	if _, err := os.Stat(cfg.DataDir); os.IsNotExist(err) {
		Error("数据目录不存在", "path", cfg.DataDir)
		Info("提示: 使用 -data 参数指定数据目录")
		return fmt.Errorf("数据目录不存在: %s", cfg.DataDir)
	}

	Info("配置信息",
		"data_dir", cfg.DataDir,
		"cache_dir", cfg.CacheDir,
		"listen_addr", cfg.ListenAddr,
	)

	// 初始化缓存
	if err := initializeCache(); err != nil {
		Warn("缓存初始化失败，将使用实时解析模式", "error", err.Error())
	}

	// 路由
	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/dashboard", dashboardPageHandler)
	mux.HandleFunc("/api/data", handleDataAPI)
	mux.HandleFunc("/api/overview", handleOverviewAPI)
	mux.HandleFunc("/api/diagnostics", handleDiagnosticsAPI)
	mux.HandleFunc("/api/detail/failures", handleDetailFailuresAPI)
	mux.HandleFunc("/api/detail/commands", handleDetailCommandsAPI)
	mux.HandleFunc("/api/detail/tokens", handleDetailTokensAPI)
	mux.HandleFunc("/api/detail/sessions", handleDetailSessionsAPI)
	mux.HandleFunc("/api/detail/tools", handleDetailToolsAPI)
	mux.HandleFunc("/api/timeline", handleTimelineAPI)
	mux.HandleFunc("/api/reload", reloadHandler)

	// 静态资源（使用嵌入的文件系统）
	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// 包装日志中间件
	handler := LoggingMiddleware(mux)

	Info("服务就绪",
		"url", "http://localhost"+cfg.ListenAddr,
		"dashboard", "http://localhost"+cfg.ListenAddr+"/dashboard",
	)
	if err := http.ListenAndServe(cfg.ListenAddr, handler); err != nil {
		Error("启动失败", "error", err.Error())
		return err
	}
	return nil
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
        <a href="/api/data?preset=30d">📡 API 接口</a>
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
    <link rel="stylesheet" href="/static/app.css?v=interactive">
</head>
<body>
    <div class="container">
        <main class="main-content">
            <div class="content-inner">
                <header class="dashboard-header">
                    <div class="page-header">
                        <p class="eyebrow">Claude Code 使用诊断</p>
                        <h1>把历史会话变成可下钻的优化证据</h1>
                        <p>统一时间范围驱动概览、失败、命令、Token、工具和 session 分析。</p>
                    </div>
                    <div class="status-strip" aria-live="polite">
                        <div><span>最后更新</span><strong id="lastUpdate">-</strong></div>
                        <div><span>数据来源</span><strong id="dataSource">-</strong></div>
                        <div><span>耗时</span><strong id="runtimeInfo">-</strong></div>
                    </div>
                </header>

                <section class="control-panel" aria-label="全局筛选">
                    <div class="control-row">
                        <div class="preset-buttons" role="group" aria-label="快捷时间范围">
                            <button class="preset-btn" data-preset="24h">24h</button>
                            <button class="preset-btn" data-preset="7d">7d</button>
                            <button class="preset-btn active" data-preset="30d">30d</button>
                            <button class="preset-btn" data-preset="90d">90d</button>
                            <button class="preset-btn" data-preset="all">All</button>
                        </div>
                        <div class="custom-range">
                            <label for="startDate">开始</label>
                            <input type="date" id="startDate">
                            <label for="endDate">结束</label>
                            <input type="date" id="endDate">
                            <button id="applyRangeBtn" type="button">应用</button>
                        </div>
                    </div>
                    <div class="timeline-control">
                        <div>
                            <label for="timelineSlider">时间轴</label>
                            <p id="timelineLabel">正在加载时间轴</p>
                        </div>
                        <input type="range" id="timelineSlider" min="0" max="0" value="0" disabled>
                        <select id="windowSize" aria-label="滑动窗口大小">
                            <option value="1">1 天</option>
                            <option value="7" selected>7 天</option>
                            <option value="30">30 天</option>
                            <option value="90">90 天</option>
                        </select>
                    </div>
                    <div class="filter-row">
                        <input id="projectFilter" type="search" placeholder="项目过滤">
                        <input id="toolFilter" type="search" placeholder="工具过滤">
                        <input id="modelFilter" type="search" placeholder="模型过滤" list="modelCandidates">
                        <datalist id="modelCandidates"></datalist>
                        <input id="reasonFilter" type="search" placeholder="失败原因过滤">
                        <button id="clearFiltersBtn" type="button">清空过滤</button>
                    </div>
                    <nav class="section-nav" aria-label="分析分组">
                        <a href="#section-overview">概览</a>
                        <a href="#section-diagnostics">诊断</a>
                        <a href="#section-details">下钻</a>
                        <a href="#section-usage">使用</a>
                        <a href="#section-quality">质量</a>
                        <a href="#section-cost">成本</a>
                        <a href="#section-runtime">运行时</a>
                    </nav>
                </section>
                <div id="errorMessage"></div>
                <div id="loadingIndicator" class="loading">
                    <div class="loading-container">
                        <div class="spinner"></div>
                        <div class="loading-text">正在加载数据</div>
                        <div class="progress-bar">
                            <div class="progress-bar-fill"></div>
                        </div>
                        <div class="loading-progress" id="loadingProgress">正在读取数据文件...</div>
                        <div class="loading-eta" id="loadingEta">预计需要 2-3 秒</div>
                        <div class="loading-tip" id="loadingTip">☕ 顺便喝口水吧~</div>
                    </div>
                </div>
                <section id="section-overview" class="summary-panel" style="display:none;">
                    <div class="section-heading">
                        <h2>概览</h2>
                        <p id="summaryRange">-</p>
                    </div>
                    <div id="summaryGrid" class="summary-grid"></div>
                </section>
                <section id="section-diagnostics" class="diagnostics-panel" style="display:none;">
                    <div class="section-heading">
                        <h2>诊断建议</h2>
                        <p>点击建议可以把下钻条件同步到详情面板。</p>
                    </div>
                    <div id="diagnosticList" class="diagnostic-list"></div>
                </section>
                <section id="section-details" class="details-panel" style="display:none;">
                    <div class="section-heading">
                        <h2>证据下钻</h2>
                        <p id="detailContext">跟随当前时间和过滤条件。</p>
                    </div>
                    <div id="detailGrid" class="detail-grid"></div>
                </section>
                <div id="chartsContainer" class="charts-container" style="display:none;"></div>
            </div>
        </main>
    </div>
    <script src="/static/echarts.min.js?v=interactive" defer></script>
    <script src="/static/app.js?v=interactive" defer></script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, templateContent)
}

// reloadHandler 重新加载数据
func reloadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, `{"status":"error","message":"method not allowed"}`)
		return
	}

	start := time.Now()
	force := r.URL.Query().Get("force") == "1" || r.URL.Query().Get("force") == "true"
	if err := refreshGlobalCache(force); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"status":"error","message":%q}`, err.Error())
		return
	}

	messages, sessions, rulesHash := 0, 0, ""
	if globalCache != nil {
		messages = globalCache.TotalMessages
		sessions = globalCache.TotalSessions
		rulesHash = globalCache.BashRulesHash
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "ok",
		"message":      "数据已刷新",
		"duration_sec": time.Since(start).Seconds(),
		"messages":     messages,
		"sessions":     sessions,
		"rules_hash":   rulesHash,
	})
}

// initializeCache 初始化缓存系统
func initializeCache() error {
	return refreshGlobalCache(false)
}
