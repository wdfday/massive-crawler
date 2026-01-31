package provider

import (
	"fmt"
	"time"
)

// TiingoProvider là implementation của DataProvider cho Tiingo API
// TODO: Implement Tiingo API crawler
type TiingoProvider struct {
	apiKey string
	// TODO: Thêm các fields cần thiết cho Tiingo
}

// NewTiingoProvider tạo Tiingo provider mới
func NewTiingoProvider(apiKey string) (*TiingoProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Tiingo API key không được để trống")
	}

	return &TiingoProvider{
		apiKey: apiKey,
	}, nil
}

// GetName trả về tên provider
func (t *TiingoProvider) GetName() string {
	return "Tiingo"
}

// CrawlMinuteBars crawl minute bars từ Tiingo API
// TODO: Implement Tiingo API integration
func (t *TiingoProvider) CrawlMinuteBars(ticker string, from, to time.Time) ([]Bar, error) {
	// TODO: Implement Tiingo API call
	return nil, fmt.Errorf("Tiingo provider chưa được implement")
}

// Close đóng các kết nối
func (t *TiingoProvider) Close() error {
	// TODO: Cleanup nếu cần
	return nil
}
