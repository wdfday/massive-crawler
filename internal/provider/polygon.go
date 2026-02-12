package provider

import (
	"us-data/internal/provider/polygon"
	"us-data/internal/saver"
)

// PolygonProvider is a DataProvider implementation backed by the Polygon API.
// It embeds *polygon.Crawler to expose crawl capabilities with minimal boilerplate.
type PolygonProvider struct {
	*polygon.Crawler
}

// NewPolygonProvider creates a new Polygon-backed DataProvider.
func NewPolygonProvider(apiKeys []string) (*PolygonProvider, error) {
	crawler, err := polygon.NewCrawler()
	if err != nil {
		return nil, err
	}
	return &PolygonProvider{
		Crawler: crawler,
	}, nil
}

// GetName returns provider name
func (p *PolygonProvider) GetName() string {
	return "Polygon"
}

// SetSavePacketDir sets directory for crawler to save packets (one file per chunk).
// Dir is typically data/Polygon. File extension depends on PacketSaver (csv/parquet/json).
func (p *PolygonProvider) SetSavePacketDir(dir string) {
	if p.Crawler != nil {
		p.Crawler.SavePacketDir = dir
	}
}

// SetPacketSaver injects packet save implementation (DIP). Call after SetSavePacketDir.
func (p *PolygonProvider) SetPacketSaver(s saver.PacketSaver) {
	if p.Crawler != nil {
		p.Crawler.PacketSaver = s
	}
}

// SetSavePerDay save per day {ticker}_{date}.ext instead of {ticker}_{from}_to_{to}.ext
func (p *PolygonProvider) SetSavePerDay(v bool) {
	if p.Crawler != nil {
		p.Crawler.SavePerDay = v
	}
}

// SetLogFunc sets fan-in logger. When set, crawler sends logs here instead of log.Printf.
func (p *PolygonProvider) SetLogFunc(fn polygon.LogFunc) {
	if p.Crawler != nil {
		p.Crawler.LogFunc = fn
	}
}
