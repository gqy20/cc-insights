package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestMainInitialization 测试主函数初始化缓存
func TestMainInitialization(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	cacheDir := filepath.Join(tmpDir, "cache")

	// 创建数据目录
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("创建数据目录失败: %v", err)
	}

	// 创建缓存目录
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("创建缓存目录失败: %v", err)
	}

	// 创建测试数据
	historyPath := filepath.Join(dataDir, "history.jsonl")
	historyContent := `{"display":"test","timestamp":` +
		strconv.FormatInt(time.Now().UnixMilli(), 10) +
		`,"project":"test-project"}`
	if err := os.WriteFile(historyPath, []byte(historyContent), 0644); err != nil {
		t.Fatalf("创建测试数据失败: %v", err)
	}

	// 设置配置
	originalDataDir := cfg.DataDir
	originalCacheDir := cfg.CacheDir
	cfg.DataDir = dataDir
	cfg.CacheDir = cacheDir
	defer func() {
		cfg.DataDir = originalDataDir
		cfg.CacheDir = originalCacheDir
	}()

	cachePath := filepath.Join(cacheDir, "cache.db")

	// Act - 初始化缓存
	builder := &CacheBuilder{
		CachePath: cachePath,
		DataDir:   dataDir,
	}
	err := builder.BuildFullCache()

	// Assert
	if err != nil {
		t.Fatalf("BuildFullCache() failed: %v", err)
	}

	// 验证缓存文件存在
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("缓存文件未创建")
	}

	// 验证缓存可以加载
	cache, err := LoadCacheFile(cachePath)
	if err != nil {
		t.Fatalf("LoadCacheFile() failed: %v", err)
	}

	if cache == nil {
		t.Fatal("加载的缓存为 nil")
	}

	if cache.TotalMessages == 0 {
		t.Error("TotalMessages 应该 > 0")
	}
}

// TestAPIUsesCachedData 测试API使用缓存数据
func TestAPIUsesCachedData(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	cacheDir := filepath.Join(tmpDir, "cache")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("创建数据目录失败: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("创建缓存目录失败: %v", err)
	}

	// 创建测试数据
	baseTime := time.Now().Add(-24 * time.Hour)
	historyPath := filepath.Join(dataDir, "history.jsonl")
	historyContent := `{"display":"test1","timestamp":` +
		strconv.FormatInt(baseTime.UnixMilli(), 10) +
		`,"project":"test-project"}`
	if err := os.WriteFile(historyPath, []byte(historyContent), 0644); err != nil {
		t.Fatalf("创建测试数据失败: %v", err)
	}

	// 设置配置
	originalDataDir := cfg.DataDir
	originalCacheDir := cfg.CacheDir
	cfg.DataDir = dataDir
	cfg.CacheDir = cacheDir
	defer func() {
		cfg.DataDir = originalDataDir
		cfg.CacheDir = originalCacheDir
	}()

	// 构建缓存
	cachePath := filepath.Join(cacheDir, "cache.db")
	builder := &CacheBuilder{
		CachePath: cachePath,
		DataDir:   dataDir,
	}
	if err := builder.BuildFullCache(); err != nil {
		t.Fatalf("Setup: BuildFullCache failed: %v", err)
	}

	// 加载缓存到全局变量（模拟main.go中的逻辑）
	cache, loadErr := LoadCacheFile(cachePath)
	if loadErr != nil {
		t.Fatalf("Setup: LoadCacheFile failed: %v", loadErr)
	}
	globalCache = cache

	// 创建API请求
	req := httptest.NewRequest("GET", "/api/data?preset=7d", nil)
	w := httptest.NewRecorder()

	// Act
	handleDataAPI(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, want %d", w.Code, http.StatusOK)
	}

	// 验证响应包含数据
	body, _ := io.ReadAll(w.Result().Body)
	defer w.Result().Body.Close()
	var response APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if !response.Success {
		t.Errorf("Success = false, error = %s", response.Error)
	}
}

// TestAPIFallbackWhenNoCache 测试无缓存时的降级处理
func TestAPIFallbackWhenNoCache(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	cacheDir := filepath.Join(tmpDir, "cache")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("创建数据目录失败: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("创建缓存目录失败: %v", err)
	}

	// 创建测试数据
	historyPath := filepath.Join(dataDir, "history.jsonl")
	historyContent := `{"display":"test","timestamp":` +
		strconv.FormatInt(time.Now().UnixMilli(), 10) +
		`,"project":"test-project"}`
	if err := os.WriteFile(historyPath, []byte(historyContent), 0644); err != nil {
		t.Fatalf("创建测试数据失败: %v", err)
	}

	// 设置配置
	originalDataDir := cfg.DataDir
	originalCacheDir := cfg.CacheDir
	cfg.DataDir = dataDir
	cfg.CacheDir = cacheDir
	defer func() {
		cfg.DataDir = originalDataDir
		cfg.CacheDir = originalCacheDir
	}()

	// 确保没有缓存
	globalCache = nil

	// 创建API请求
	req := httptest.NewRequest("GET", "/api/data?preset=all", nil)
	w := httptest.NewRecorder()

	// Act
	handleDataAPI(w, req)

	// Assert - 应该降级到实时解析
	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, want %d", w.Code, http.StatusOK)
	}

	body, _ := io.ReadAll(w.Result().Body)
	defer w.Result().Body.Close()
	var response APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if !response.Success {
		t.Errorf("Success = false, error = %s", response.Error)
	}
}

// TestIncrementalCacheUpdate 测试增量更新
func TestIncrementalCacheUpdate(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	cacheDir := filepath.Join(tmpDir, "cache")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("创建数据目录失败: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("创建缓存目录失败: %v", err)
	}

	// 创建初始数据
	baseTime := time.Now().Add(-2 * 24 * time.Hour)
	historyPath := filepath.Join(dataDir, "history.jsonl")
	initialContent := `{"display":"old","timestamp":` +
		strconv.FormatInt(baseTime.UnixMilli(), 10) +
		`,"project":"test"}`
	if err := os.WriteFile(historyPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("创建初始数据失败: %v", err)
	}

	// 设置配置
	originalDataDir := cfg.DataDir
	originalCacheDir := cfg.CacheDir
	cfg.DataDir = dataDir
	cfg.CacheDir = cacheDir
	defer func() {
		cfg.DataDir = originalDataDir
		cfg.CacheDir = originalCacheDir
	}()

	cachePath := filepath.Join(cacheDir, "cache.db")

	// 第一次构建
	builder := &CacheBuilder{
		CachePath: cachePath,
		DataDir:   dataDir,
	}
	if err := builder.BuildFullCache(); err != nil {
		t.Fatalf("Setup: 初始构建失败: %v", err)
	}

	initialCache, _ := LoadCacheFile(cachePath)
	initialMessageCount := initialCache.TotalMessages

	// 等待1秒确保文件修改时间不同
	time.Sleep(1 * time.Second)

	// 添加新数据
	newTime := time.Now().Add(1 * time.Minute)
	newContent := initialContent + `
{"display":"new1","timestamp":` + strconv.FormatInt(newTime.UnixMilli(), 10) + `,"project":"test"}
{"display":"new2","timestamp":` + strconv.FormatInt(newTime.Add(time.Second).UnixMilli(), 10) + `,"project":"test"}`
	if err := os.WriteFile(historyPath, []byte(newContent), 0644); err != nil {
		t.Fatalf("添加新数据失败: %v", err)
	}

	// Act - 增量更新
	updateErr := builder.IncrementalUpdate()

	// Assert
	if updateErr != nil {
		t.Fatalf("IncrementalUpdate() failed: %v", updateErr)
	}

	updatedCache, loadErr := LoadCacheFile(cachePath)
	if loadErr != nil {
		t.Fatalf("加载更新后的缓存失败: %v", loadErr)
	}

	if updatedCache.TotalMessages <= initialMessageCount {
		t.Errorf("TotalMessages = %d, should be > %d",
			updatedCache.TotalMessages, initialMessageCount)
	}

	if !updatedCache.LastUpdate.After(initialCache.LastUpdate) {
		t.Error("LastUpdate 应该晚于初始缓存时间")
	}
}
