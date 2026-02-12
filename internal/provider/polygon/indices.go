package polygon

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// LoadTickersFromFile reads a list of tickers from a file.
// Supported formats:
//   - .txt  : one ticker per line, '#' lines are treated as comments
//   - .json : JSON array of strings
func LoadTickersFromFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file %s: %w", path, err)
	}
	defer file.Close()

	// Read full content
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var tickers []string

	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(content, &tickers); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
	case ".txt":
		tickers = parseTickersFromText(string(content))
	default:
		return nil, fmt.Errorf("unsupported ticker file extension %q (use .txt or .json)", filepath.Ext(path))
	}

	// Remove empty and duplicates
	seen := make(map[string]bool)
	var uniqueTickers []string
	for _, t := range tickers {
		t = strings.TrimSpace(strings.ToUpper(t))
		if t != "" && !seen[t] {
			seen[t] = true
			uniqueTickers = append(uniqueTickers, t)
		}
	}

	slog.Info("loaded tickers from file", "count", len(uniqueTickers), "path", path)
	return uniqueTickers, nil
}

// parseTickersFromText parses a plain text representation of tickers
// where each non-empty, non-comment line represents a ticker.
func parseTickersFromText(s string) []string {
	lines := strings.Split(s, "\n")
	var tickers []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			tickers = append(tickers, line)
		}
	}
	return tickers
}

// LoadTickersFromIndicesFile reads from indices/combined.txt or tickers.json
func LoadTickersFromIndicesFile() ([]string, error) {
	// Try possible paths
	possiblePaths := []string{
		"indices/combined.txt",
		"indices/tickers.json",
		"indices/sp500.txt",
	}

	for _, path := range possiblePaths {
		// Resolve relative path
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}

		if _, err := os.Stat(absPath); err == nil {
			slog.Info("found indices file", "path", absPath)
			return LoadTickersFromFile(absPath)
		}
	}

	return nil, fmt.Errorf("indices file not found. Run scripts/fetch_indices.sh first")
}

// LoadTickersFromFileOrIndices tries to load from specified file, falls back to indices
func LoadTickersFromFileOrIndices(filepath string) ([]string, error) {
	if filepath != "" {
		// Try load from specified file
		if _, err := os.Stat(filepath); err == nil {
			return LoadTickersFromFile(filepath)
		}
		slog.Info("file not found, trying indices", "path", filepath)
	}

	// Fallback to indices
	return LoadTickersFromIndicesFile()
}
