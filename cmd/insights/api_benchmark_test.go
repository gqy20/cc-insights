package main

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkBuildDataFromParsingAll(b *testing.B) {
	dataDir := filepath.Join(os.Getenv("HOME"), ".claude")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		b.Skip("skip: ~/.claude data directory does not exist")
	}

	origDataDir := cfg.DataDir
	cfg.DataDir = dataDir
	defer func() { cfg.DataDir = origDataDir }()

	tf := TimeFilter{Start: nil, End: nil}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := buildDataFromParsing(tf, "all"); err != nil {
			b.Fatalf("buildDataFromParsing failed: %v", err)
		}
	}
}
