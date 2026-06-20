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

//go:embed static/dist
var distFS embed.FS

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
	mux.HandleFunc("/dashboard/", dashboardPageHandler)
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
	mux.HandleFunc("/api/version", versionHandler)

	// 静态资源：React 构建产物（cmd/insights/static/dist），由 web/ 经 Vite 生成后 embed。
	distSub, _ := fs.Sub(distFS, "static/dist")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(distSub))))

	// 包装日志中间件
	handler := LoggingMiddleware(mux)

	Info("服务就绪",
		"url", "http://localhost"+cfg.ListenAddr,
		"dashboard", "http://localhost"+cfg.ListenAddr+"/dashboard",
	)
	// 枚举局域网/Tailscale/ZeroTier 等外部可达地址，方便分享给同网段或 overlay 内的设备。
	for _, u := range accessibleDashboardURLs(cfg.ListenAddr) {
		Info("可访问地址", u.Iface, u.URL)
	}
	if err := http.ListenAndServe(cfg.ListenAddr, handler); err != nil {
		Error("启动失败", "error", err.Error())
		return err
	}
	return nil
}

// indexHandler 根路径重定向到 Dashboard SPA，入口统一。
func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// dashboardPageHandler 返回 React SPA 入口（cmd/insights/static/dist/index.html）。
// /dashboard 与 /dashboard/* 都回退到该入口，兼容前端客户端路由。
func dashboardPageHandler(w http.ResponseWriter, r *http.Request) {
	indexBytes, err := distIndexBytes()
	if err != nil {
		http.Error(w, "Dashboard 资源缺失，请先构建前端 (make web-build)", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexBytes)
}

// distIndexBytes 读取嵌入的 React 构建产物 index.html。
func distIndexBytes() ([]byte, error) {
	return fs.ReadFile(distFS, "static/dist/index.html")
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

// versionHandler 返回构建版本信息（version/commit/buildDate 由 -ldflags -X 注入）。
func versionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"version":   version,
		"commit":    commit,
		"buildDate": buildDate,
	})
}

// initializeCache 初始化缓存系统
func initializeCache() error {
	return refreshGlobalCache(false)
}
