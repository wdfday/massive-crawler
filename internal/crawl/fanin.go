package crawl

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

func runLogWriter(lines <-chan string) {
	for s := range lines {
		fmt.Println(s)
	}
}

type errorEntry struct {
	Ticker string
	Err    error
}

func runErrorHandler(errors <-chan errorEntry, logger *slog.Logger) {
	for e := range errors {
		logger.Error("crawl error", "ticker", e.Ticker, "error", e.Err)
	}
}

func runHeartbeat(ctx context.Context, interval time.Duration, totalJobs int, mu *sync.Mutex, success, failed *int, barsPerTicker map[string]int, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mu.Lock()
			s, f := *success, *failed
			var totalBars int
			for _, n := range barsPerTicker {
				totalBars += n
			}
			mu.Unlock()
			logger.Info("heartbeat", "done", s+f, "total", totalJobs, "success", s, "failed", f, "bars", totalBars)
		}
	}
}
