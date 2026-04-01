package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestIncrementalParser_NeedsParse(t *testing.T) {
	p := NewTSParser()
	cacheDir := filepath.Join(os.TempDir(), "incremental_test_cache")
	defer os.RemoveAll(cacheDir)

	ip := NewIncrementalParser(p, filepath.Join(cacheDir, "cache.json"))

	content := []byte("function hello() { return 'world'; }")

	if !ip.NeedsParse("test.js", content) {
		t.Error("expected NeedsParse to return true for new file")
	}

	ctx := context.Background()
	_, err := ip.ParseIncremental(ctx, []ParseChange{
		{Path: "test.js", Content: content},
	})
	if err != nil {
		t.Fatalf("ParseIncremental failed: %v", err)
	}

	if ip.NeedsParse("test.js", content) {
		t.Error("expected NeedsParse to return false for unchanged file")
	}

	modified := []byte("function hello() { return 'modified'; }")
	if !ip.NeedsParse("test.js", modified) {
		t.Error("expected NeedsParse to return true for modified file")
	}
}

func TestIncrementalParser_ParseIncremental(t *testing.T) {
	p := NewTSParser()
	cacheDir := filepath.Join(os.TempDir(), "incremental_parse_test")
	defer os.RemoveAll(cacheDir)

	ip := NewIncrementalParser(p, filepath.Join(cacheDir, "cache.json"))
	ctx := context.Background()

	goContent := []byte("package main\nfunc main() {}")
	tsContent := []byte("interface User { id: number; }")

	result, err := ip.ParseIncremental(ctx, []ParseChange{
		{Path: "main.go", Content: goContent},
		{Path: "user.ts", Content: tsContent},
	})
	if err != nil {
		t.Fatalf("ParseIncremental failed: %v", err)
	}

	if len(result.ParsedFiles) != 2 {
		t.Errorf("expected 2 parsed files, got %d", len(result.ParsedFiles))
	}

	if len(result.CachedFiles) != 0 {
		t.Errorf("expected 0 cached files, got %d", len(result.CachedFiles))
	}

	foundGo := false
	foundTs := false
	for _, f := range result.ParsedFiles {
		if f == "main.go" {
			foundGo = true
		}
		if f == "user.ts" {
			foundTs = true
		}
	}

	if !foundGo {
		t.Error("Go file not parsed")
	}
	if !foundTs {
		t.Error("TypeScript file not parsed")
	}

	result2, err := ip.ParseIncremental(ctx, []ParseChange{
		{Path: "main.go", Content: goContent},
		{Path: "user.ts", Content: tsContent},
	})
	if err != nil {
		t.Fatalf("Second ParseIncremental failed: %v", err)
	}

	if len(result2.CachedFiles) != 2 {
		t.Errorf("expected 2 cached files, got %d", len(result2.CachedFiles))
	}
}

func TestIncrementalParser_SkipUnsupported(t *testing.T) {
	p := NewTSParser()
	cacheDir := filepath.Join(os.TempDir(), "incremental_unsupported_test")
	defer os.RemoveAll(cacheDir)

	ip := NewIncrementalParser(p, filepath.Join(cacheDir, "cache.json"))
	ctx := context.Background()

	content := []byte("some random content")

	result, err := ip.ParseIncremental(ctx, []ParseChange{
		{Path: "test.txt", Content: content},
		{Path: "readme.md", Content: content},
	})
	if err != nil {
		t.Fatalf("ParseIncremental failed: %v", err)
	}

	if len(result.ParsedFiles) != 0 {
		t.Errorf("expected 0 parsed files for unsupported extensions, got %d", len(result.ParsedFiles))
	}
}

func TestIncrementalParser_CacheStats(t *testing.T) {
	p := NewTSParser()
	cacheDir := filepath.Join(os.TempDir(), "incremental_stats_test")
	defer os.RemoveAll(cacheDir)

	ip := NewIncrementalParser(p, filepath.Join(cacheDir, "cache.json"))
	ctx := context.Background()

	content := []byte("func hello() {}")

	_, err := ip.ParseIncremental(ctx, []ParseChange{
		{Path: "test.go", Content: content},
	})
	if err != nil {
		t.Fatalf("ParseIncremental failed: %v", err)
	}

	stats := ip.GetCacheStats()
	cachedFiles, ok := stats["cached_files"].(int)
	if !ok {
		t.Fatal("cached_files not found in stats")
	}

	if cachedFiles != 1 {
		t.Errorf("expected 1 cached file, got %d", cachedFiles)
	}
}

func TestIncrementalParser_ParseFull(t *testing.T) {
	p := NewTSParser()
	cacheDir := filepath.Join(os.TempDir(), "incremental_full_test")
	defer os.RemoveAll(cacheDir)

	ip := NewIncrementalParser(p, filepath.Join(cacheDir, "cache.json"))
	ctx := context.Background()

	content1 := []byte("func first() {}")
	content2 := []byte("func second() {}")

	_, err := ip.ParseIncremental(ctx, []ParseChange{
		{Path: "first.go", Content: content1},
	})
	if err != nil {
		t.Fatalf("First ParseIncremental failed: %v", err)
	}

	result, err := ip.ParseFull(ctx, []ParseChange{
		{Path: "first.go", Content: content1},
		{Path: "second.go", Content: content2},
	})
	if err != nil {
		t.Fatalf("ParseFull failed: %v", err)
	}

	if len(result.ParsedFiles) != 2 {
		t.Errorf("expected 2 parsed files, got %d", len(result.ParsedFiles))
	}

	if len(result.CachedFiles) != 0 {
		t.Errorf("expected 0 cached files after full reparse, got %d", len(result.CachedFiles))
	}
}

func TestIncrementalResult_HasErrors(t *testing.T) {
	result := &IncrementalResult{
		AllErrors: []ParseError{{Message: "test error"}},
	}

	if !result.HasErrors() {
		t.Error("expected HasErrors to return true")
	}

	result.AllErrors = nil
	if result.HasErrors() {
		t.Error("expected HasErrors to return false")
	}
}
