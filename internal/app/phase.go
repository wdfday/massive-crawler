package app

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"us-data/internal/crawl"
	"us-data/internal/provider"
)

// RunFlow orchestrates crawl loop: trigger → run → done → wait → trigger
func RunFlow(cfg *Config, dp provider.DataProvider, tickers []string) {
	progressUpdates := make(chan crawl.ProgressUpdate, 256)
	go crawl.RunProgressWriter(cfg.ProgressPath(), progressUpdates)

	shutdown := make(chan struct{})
	trigger := make(chan crawl.Cmd, 1)
	done := make(chan crawl.Done, 1)

	go func() {
		for range trigger {
			crawl.RunOneCrawl(
				dp,
				cfg.PolygonAPIKeys,
				tickers,
				cfg.SaveBaseDir(),
				cfg.SaveFormat,
				cfg.ProgressPath(),
				progressUpdates,
				done,
				shutdown,
			)
		}
	}()

	trigger <- crawl.Cmd{}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-done:
			slog.Info("done, wait until next run")
			nextRun := nextCrawlRunTime(cfg)
			waitDur := time.Until(nextRun)
			if waitDur <= 0 {
				slog.Info("next run passed, running now", "next_run", nextRun.Format("2006-01-02 15:04"))
			} else {
				slog.Info("timer waiting", "hours", waitDur.Hours(), "until", nextRun.Format("2006-01-02 15:04"))
				timer := time.NewTimer(waitDur)
				select {
				case <-timer.C:
				case sig := <-signals:
					slog.Info("received signal, stopping", "sig", sig, "restart_at", nextRun.Format("2006-01-02 15:04"))
					timer.Stop()
					return
				}
			}
			trigger <- crawl.Cmd{}
		case sig := <-signals:
			slog.Info("received signal, graceful shutdown", "sig", sig)
			close(shutdown)
			<-done
			return
		}
	}
}

func nextCrawlRunTime(cfg *Config) time.Time {
	now := time.Now().UTC()
	hour, min := cfg.Phase2RunHour, cfg.Phase2RunMinute
	targetToday := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, time.UTC)
	if now.Before(targetToday) {
		return targetToday
	}
	tomorrow := now.AddDate(0, 0, 1)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), hour, min, 0, 0, time.UTC)
}
