package provider

import (
	"time"

	"us-data/polygon"
	"us-data/saver"
)

// PolygonProvider là implementation của DataProvider cho Polygon API
type PolygonProvider struct {
	crawler *polygon.Crawler
}

// NewPolygonProvider tạo Polygon provider mới
func NewPolygonProvider(apiKeys []string, strategy polygon.KeySelectionStrategy) (*PolygonProvider, error) {
	var crawler *polygon.Crawler
	var err error

	if len(apiKeys) == 1 {
		crawler, err = polygon.NewCrawler(apiKeys[0])
	} else {
		crawler, err = polygon.NewCrawlerWithKeyPool(apiKeys, strategy)
	}

	if err != nil {
		return nil, err
	}

	return &PolygonProvider{
		crawler: crawler,
	}, nil
}

// GetName trả về tên provider
func (p *PolygonProvider) GetName() string {
	return "Polygon"
}

// SetSavePacketDir set thư mục để crawler lưu từng packet (mỗi chunk 1 file).
// Dir thường là data/Polygon. Extension file phụ thuộc vào PacketSaver (csv/parquet/json).
func (p *PolygonProvider) SetSavePacketDir(dir string) {
	if p.crawler != nil {
		p.crawler.SavePacketDir = dir
	}
}

// SetPacketSaver inject implementation lưu packet (DIP). Gọi sau SetSavePacketDir.
func (p *PolygonProvider) SetPacketSaver(s saver.PacketSaver) {
	if p.crawler != nil {
		p.crawler.PacketSaver = s
	}
}

// CrawlMinuteBars crawl minute bars từ Polygon API
func (p *PolygonProvider) CrawlMinuteBars(ticker string, from, to time.Time) ([]Bar, error) {
	polygonBars, err := p.crawler.CrawlMinuteBars(ticker, from, to)
	if err != nil {
		return nil, err
	}

	// Convert từ polygon.Bar sang provider.Bar
	bars := make([]Bar, len(polygonBars))
	for i, pb := range polygonBars {
		bars[i] = Bar{
			Timestamp:    pb.Timestamp,
			Open:         pb.Open,
			High:         pb.High,
			Low:          pb.Low,
			Close:        pb.Close,
			Volume:       pb.Volume,
			VWAP:         pb.VWAP,
			Transactions: pb.Transactions,
		}
	}

	return bars, nil
}

// Close đóng các kết nối
func (p *PolygonProvider) Close() error {
	if p.crawler != nil {
		return p.crawler.Close()
	}
	return nil
}
