package app

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"us-data/internal/provider"
	"us-data/internal/provider/polygon"
	"us-data/internal/saver"
)

// CreateProvider creates DataProvider from config (currently Polygon only)
func CreateProvider(cfg *Config) (provider.DataProvider, error) {
	switch strings.ToLower(cfg.DataProvider) {
	case "polygon":
		return createPolygonProvider(cfg)
	default:
		return nil, fmt.Errorf("unsupported data provider: %s. Options: polygon", cfg.DataProvider)
	}
}

func createPolygonProvider(cfg *Config) (provider.DataProvider, error) {
	if len(cfg.PolygonAPIKeys) == 0 {
		return nil, fmt.Errorf("POLYGON_API_KEY or POLYGON_API_KEYS not set")
	}
	return provider.NewPolygonProvider(cfg.PolygonAPIKeys)
}

// CreatePolygonCrawler creates a polygon.Crawler used for loading ticker lists (e.g. index constituents).
func CreatePolygonCrawler(cfg *Config) (*polygon.Crawler, error) {
	if len(cfg.PolygonAPIKeys) == 0 {
		return nil, fmt.Errorf("POLYGON_API_KEY or POLYGON_API_KEYS not set")
	}
	return polygon.NewCrawler()
}

// WirePolygonPacketSave injects PacketSaver into Polygon provider
func WirePolygonPacketSave(dp provider.DataProvider, saveBaseDir, saveFormat string) {
	p, ok := dp.(*provider.PolygonProvider)
	if !ok {
		return
	}
	packetSaver := saver.NewPacketSaver(saveFormat)
	if packetSaver == nil {
		slog.Error("invalid SAVE_FORMAT", "format", saveFormat, "allowed", "csv, parquet, json")
		os.Exit(1)
	}
	p.SetSavePacketDir(saveBaseDir)
	p.SetPacketSaver(packetSaver)
	slog.Info("wire", "provider", "Polygon", "format", saveFormat, "dir", saveBaseDir, "pattern", "{Ticker}/{ticker}_{from}_to_{to}."+packetSaver.Extension())
}
