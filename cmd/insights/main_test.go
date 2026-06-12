package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestMainInitialization 测试主函数初始化缓存
func TestMainInitialization(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	dataDir := createTestDataDir(t, tmpDir)
	cacheDir := t.TempDir()
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
	dataDir := createTestDataDir(t, tmpDir)
	cacheDir := t.TempDir()
	cachePath := filepath.Join(cacheDir, "cache.db")

	// 设置配置
	originalDataDir := cfg.DataDir
	originalCacheDir := cfg.CacheDir
	originalGlobalCache := globalCache
	cfg.DataDir = dataDir
	cfg.CacheDir = cacheDir
	defer func() {
		cfg.DataDir = originalDataDir
		cfg.CacheDir = originalCacheDir
		globalCache = originalGlobalCache
	}()

	// 构建缓存
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

	// 设置配置
	originalDataDir := cfg.DataDir
	originalCacheDir := cfg.CacheDir
	originalGlobalCache := globalCache
	cfg.DataDir = dataDir
	defer func() {
		cfg.DataDir = originalDataDir
		cfg.CacheDir = originalCacheDir
		globalCache = originalGlobalCache
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

// TestReloadHandlerForcesCacheRefresh 测试 reload 接口会刷新缓存
func TestReloadHandlerForcesCacheRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := createTestDataDir(t, tmpDir)
	cacheDir := t.TempDir()
	rulesPath := filepath.Join(tmpDir, "bash.yml")

	if err := os.WriteFile(rulesPath, []byte("version: 1\nfamilies:\n  - name: custom\n    commands:\n      - foo\n"), 0644); err != nil {
		t.Fatalf("创建规则文件失败: %v", err)
	}

	originalDataDir := cfg.DataDir
	originalCacheDir := cfg.CacheDir
	originalRulesPath := cfg.RulesPath
	originalGlobalCache := globalCache
	cfg.DataDir = dataDir
	cfg.CacheDir = cacheDir
	cfg.RulesPath = ""
	defer func() {
		cfg.DataDir = originalDataDir
		cfg.CacheDir = originalCacheDir
		cfg.RulesPath = originalRulesPath
		globalCache = originalGlobalCache
	}()

	if err := refreshGlobalCache(false); err != nil {
		t.Fatalf("初始化缓存失败: %v", err)
	}
	oldHash := globalCache.BashRulesHash

	cfg.RulesPath = rulesPath
	req := httptest.NewRequest(http.MethodPost, "/api/reload?force=1", nil)
	w := httptest.NewRecorder()

	reloadHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, want %d", w.Code, http.StatusOK)
	}
	if globalCache == nil {
		t.Fatal("globalCache should not be nil")
	}
	if globalCache.BashRulesHash == oldHash {
		t.Fatalf("BashRulesHash did not change after reload")
	}
}
