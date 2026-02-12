package crawl

import (
	"encoding/json"
	"log/slog"
	"os"
)

// ProgressUpdate is sent when a ticker crawl succeeds
type ProgressUpdate struct {
	Ticker string
	Date   string
}

func loadProgress(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]string)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return make(map[string]string)
	}
	return m
}

// RunProgressWriter receives updates and persists to file (run as goroutine)
func RunProgressWriter(path string, updates <-chan ProgressUpdate) {
	m := loadProgress(path)
	for u := range updates {
		m[u.Ticker] = u.Date
		data, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			slog.Warn("progress marshal error", "error", err)
			continue
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			slog.Warn("progress write error", "error", err)
		}
	}
}
