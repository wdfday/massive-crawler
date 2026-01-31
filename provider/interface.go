package provider

import (
	"time"
)

// Bar đại diện cho một bar/candle trong dữ liệu aggregates
type Bar struct {
	Timestamp    int64   `json:"t"` // Unix timestamp in milliseconds
	Open         float64 `json:"o"`
	High         float64 `json:"h"`
	Low          float64 `json:"l"`
	Close        float64 `json:"c"`
	Volume       int64   `json:"v"`
	VWAP         float64 `json:"vw,omitempty"` // Volume weighted average price
	Transactions int64   `json:"n,omitempty"`  // Number of transactions
}

// DataProvider định nghĩa interface cho các data provider
type DataProvider interface {
	// CrawlMinuteBars crawl minute bars cho một ticker trong khoảng thời gian
	CrawlMinuteBars(ticker string, from, to time.Time) ([]Bar, error)
	
	// GetName trả về tên của provider
	GetName() string
	
	// Close đóng các kết nối
	Close() error
}

// RateLimiter interface cho rate limiting
type RateLimiter interface {
	Wait() error
	WaitForKey(key string) error
	Close() error
}

// NextURLStorage interface cho lưu trữ next_url
type NextURLStorage interface {
	SaveNextURL(key string, url string) error
	GetNextURL(key string) (string, error)
	DeleteNextURL(key string) error
}
