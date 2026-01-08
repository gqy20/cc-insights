package main

import (
	"flag"
	"os"
	"path/filepath"
)

// Config 应用配置
type Config struct {
	DataDir    string
	ListenAddr string
	BaseURL    string
}

var cfg Config

func init() {
	// 获取当前可执行文件所在目录
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	exeDir := filepath.Dir(exePath)

	// 默认数据目录（相对于可执行文件）
	defaultDataDir := filepath.Join(exeDir, "data")

	flag.StringVar(&cfg.DataDir, "data", defaultDataDir, "数据目录路径")
	flag.StringVar(&cfg.ListenAddr, "addr", ":8080", "监听地址")
	flag.StringVar(&cfg.BaseURL, "base", "", "基础URL（用于反向代理）")
}

// GetDataPath 获取数据文件路径
func GetDataPath(relPath ...string) string {
	paths := append([]string{cfg.DataDir}, relPath...)
	return filepath.Join(paths...)
}
