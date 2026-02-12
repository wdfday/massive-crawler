package main

import (
	"log/slog"
	"os"

	"us-data/internal/app"
	"us-data/internal/provider/polygon"
	"us-data/internal/slogx"
)

const keyCooldownSec = 12

func init() {
	slog.SetDefault(slogx.NewDefault("info"))
}

func main() {
	a, err := InitializeApp()
	if err != nil {
		slog.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}
	defer a.DP.Close()

	cfg := a.Config
	dp := a.DP

	slog.SetDefault(slogx.NewDefault(cfg.LogLevel))
	slog.Info("using data provider", "provider", dp.GetName())

	tickers, err := loadTickers(cfg)
	if err != nil {
		slog.Error("failed to get tickers", "error", err)
		os.Exit(1)
	}
	slog.Info("got tickers", "count", len(tickers))

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		slog.Error("failed to create data dir", "error", err)
		os.Exit(1)
	}
	slog.Info("save dir", "dir", cfg.SaveBaseDir(), "format", cfg.SaveFormat)

	slog.Info("parallel mode", "workers", len(cfg.PolygonAPIKeys), "keys", len(cfg.PolygonAPIKeys), "cooldown_sec", keyCooldownSec)

	app.RunFlow(cfg, dp, tickers)
}

func loadTickers(cfg *app.Config) ([]string, error) {
	slog.Info("reading tickers from file")
	if cfg.TickersFile != "" {
		return polygon.LoadTickersFromFileOrIndices(cfg.TickersFile)
	}
	return polygon.LoadTickersFromIndicesFile()
}
