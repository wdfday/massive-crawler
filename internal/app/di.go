package app

import (
	"fmt"

	"us-data/internal/provider"
	"us-data/internal/provider/polygon"
	"us-data/internal/saver"
)

// ProvideConfig loads config from environment (for Wire).
func ProvideConfig() *Config {
	return LoadConfig()
}

// ProvidePacketSaver creates PacketSaver from config (for Wire).
// Returns error if SaveFormat is not supported.
func ProvidePacketSaver(cfg *Config) (saver.PacketSaver, error) {
	ps := saver.NewPacketSaver(cfg.SaveFormat)
	if ps == nil {
		return nil, fmt.Errorf("unsupported SAVE_FORMAT %q (use: csv, parquet, json)", cfg.SaveFormat)
	}
	return ps, nil
}

// ProvidePolygonProvider creates and wires PolygonProvider with config and PacketSaver (for Wire).
// Caller must call dp.Close() when shutting down.
func ProvidePolygonProvider(cfg *Config, ps saver.PacketSaver) (*provider.PolygonProvider, error) {
	p, err := createPolygonProvider(cfg)
	if err != nil {
		return nil, err
	}
	pp, ok := p.(*provider.PolygonProvider)
	if !ok {
		return nil, fmt.Errorf("expected *provider.PolygonProvider, got %T", p)
	}
	pp.SetSavePacketDir(cfg.SaveBaseDir())
	pp.SetPacketSaver(ps)
	return pp, nil
}

// ProvidePolygonCrawler creates polygon.Crawler for ticker loading (for Wire / optional use).
func ProvidePolygonCrawler(cfg *Config) (*polygon.Crawler, error) {
	return CreatePolygonCrawler(cfg)
}
