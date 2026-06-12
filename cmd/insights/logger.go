package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

var levelNames = map[LogLevel]string{
	LogLevelDebug: "DEBUG",
	LogLevelInfo:  "INFO ",
	LogLevelWarn:  "WARN ",
	LogLevelError: "ERROR",
}

// Logger 结构化日志器，同时输出到 stderr 和日志文件
type Logger struct {
	level      LogLevel
	mu         sync.Mutex
	file       *os.File
	fileLogger *log.Logger
	outLogger  *log.Logger
}

// 全局日志实例
var appLogger *Logger

// InitLogger 初始化日志系统，输出到 stderr + ~/.cc-insights/logs/ 目录
func InitLogger(logDir string) error {
	appLogger = &Logger{
		level:     LogLevelInfo,
		outLogger: log.New(os.Stderr, "", 0),
	}

	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 按日期命名日志文件：cc-insights-2026-06-11.log
	dateStr := time.Now().Format("2006-01-02")
	logFile := filepath.Join(logDir, fmt.Sprintf("cc-insights-%s.log", dateStr))

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}
	appLogger.file = f

	// 文件日志只写文件；stderr 由 outLogger 单独负责。
	appLogger.fileLogger = log.New(f, "", 0)

	Info("日志系统初始化完成", "log_file", logFile)
	return nil
}

// Close 关闭日志文件
func CloseLogger() {
	if appLogger != nil && appLogger.file != nil {
		appLogger.file.Close()
	}
}

// SetLevel 动态调整日志级别（运行时可通过 API 切换）
func SetLevel(level LogLevel) {
	if appLogger != nil {
		appLogger.mu.Lock()
		appLogger.level = level
		appLogger.mu.Unlock()
	}
}

// --- 日志方法 ---

func (l *Logger) log(level LogLevel, msg string, pairs ...any) {
	if l == nil || level < l.level {
		return
	}
	prefix := fmt.Sprintf("[%s] %s |", time.Now().Format("15:04:05"), levelNames[level])
	formatted := formatMessage(msg, pairs...)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.outLogger != nil {
		l.outLogger.Output(2, prefix+" "+formatted)
	}
	if l.fileLogger != nil {
		l.fileLogger.Output(3, prefix+" "+formatted) // 更深调用栈以区分来源
	}
}

func Debug(msg string, pairs ...any) { appLogger.log(LogLevelDebug, msg, pairs...) }
func Info(msg string, pairs ...any)  { appLogger.log(LogLevelInfo, msg, pairs...) }
func Warn(msg string, pairs ...any)  { appLogger.log(LogLevelWarn, msg, pairs...) }
func Error(msg string, pairs ...any) { appLogger.log(LogLevelError, msg, pairs...) }

// RequestLog 记录 HTTP 请求（访问日志）
func RequestLog(r *http.Request, status int, duration time.Duration, size int64) {
	if appLogger == nil {
		return
	}
	method := r.Method
	path := r.URL.Path
	query := r.URL.RawQuery
	if len(query) > 100 {
		query = query[:100] + "..."
	}
	clientIP := r.RemoteAddr
	userAgent := r.UserAgent()
	if len(userAgent) > 80 {
		userAgent = userAgent[:80] + "..."
	}

	target := path
	if query != "" {
		target += "?" + query
	}

	msg := fmt.Sprintf("%s %s -> %d (%s, %dB)", method, target, status, duration.Round(time.Millisecond), size)
	appLogger.log(LogLevelInfo, msg,
		"client", clientIP,
		"ua", userAgent,
	)
}

// formatMessage 将 key-value 对格式化为 "key=value" 后缀
func formatMessage(msg string, pairs ...any) string {
	if len(pairs) == 0 {
		return msg
	}
	result := msg
	for i := 0; i < len(pairs); i += 2 {
		key := "?"
		val := "?"
		if i < len(pairs) {
			if k, ok := pairs[i].(string); ok {
				key = k
			} else {
				key = fmt.Sprintf("%v", pairs[i])
			}
		}
		if i+1 < len(pairs) {
			val = fmt.Sprintf("%v", pairs[i+1])
		}
		result += fmt.Sprintf(" %s=%s", key, val)
	}
	return result
}

// LoggingMiddleware HTTP 请求日志中间件
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 包装 ResponseWriter 以捕获状态码和响应大小
		rw := &responseWriter{ResponseWriter: w, status: 200}

		next.ServeHTTP(rw, r)

		RequestLog(r, rw.status, time.Since(start), rw.written)
	})
}

// responseWriter 包装 http.ResponseWriter 以捕获状态码和写入字节数
type responseWriter struct {
	http.ResponseWriter
	status  int
	written int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}
