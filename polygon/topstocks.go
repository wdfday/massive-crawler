package polygon

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
)

// MarketSnapshotResponse là response từ Full Market Snapshot API
type MarketSnapshotResponse struct {
	Status    string                 `json:"status"`
	RequestID string                 `json:"request_id,omitempty"`
	Results   []MarketSnapshotTicker `json:"results"`
}

// MarketSnapshotTicker chứa thông tin ticker từ market snapshot
type MarketSnapshotTicker struct {
	Ticker          string `json:"T"`
	Name            string `json:"name,omitempty"`
	Market          string `json:"market,omitempty"`
	Locale          string `json:"locale,omitempty"`
	PrimaryExchange string `json:"primary_exchange,omitempty"`
	Type            string `json:"type,omitempty"`
	Active          bool   `json:"active,omitempty"`
	Currency        string `json:"currency_name,omitempty"`

	// Snapshot data
	Day struct {
		Open   float64 `json:"o"`
		High   float64 `json:"h"`
		Low    float64 `json:"l"`
		Close  float64 `json:"c"`
		Volume int64   `json:"v"`
		VWAP   float64 `json:"vw"`
	} `json:"day"`

	PrevDay struct {
		Open   float64 `json:"o"`
		High   float64 `json:"h"`
		Low    float64 `json:"l"`
		Close  float64 `json:"c"`
		Volume int64   `json:"v"`
		VWAP   float64 `json:"vw"`
	} `json:"prevDay"`

	// Market cap (nếu có)
	MarketCap float64 `json:"market_cap,omitempty"`
}

// SortBy là enum cho cách sắp xếp
type SortBy int

const (
	SortByVolume SortBy = iota
	SortByMarketCap
	SortByVWAP
)

// GetTopStocks lấy top N stocks từ Full Market Snapshot và sort theo tiêu chí
func (c *Crawler) GetTopStocks(count int, sortBy SortBy) ([]string, error) {
	// Đợi rate limiter và lấy API key
	apiKey, err := c.getAPIKey()
	if err != nil {
		return nil, fmt.Errorf("lỗi khi lấy API key: %w", err)
	}

	// Gọi Full Market Snapshot API
	requestURL := fmt.Sprintf("%s/v2/snapshot/locale/us/markets/stocks/tickers?apiKey=%s",
		polygonBaseURL, apiKey)

	log.Printf("Đang lấy Full Market Snapshot...")

	resp, err := c.httpClient.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi gọi API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API trả về status %d: %s", resp.StatusCode, string(body))
	}

	var result MarketSnapshotResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("lỗi khi parse JSON: %w", err)
	}

	if result.Status != "OK" {
		return nil, fmt.Errorf("API trả về status không OK: %s", result.Status)
	}

	log.Printf("Đã nhận được %d tickers từ market snapshot", len(result.Results))

	// Filter chỉ lấy active stocks
	var activeTickers []MarketSnapshotTicker
	for _, ticker := range result.Results {
		if ticker.Active && ticker.Market == "stocks" {
			activeTickers = append(activeTickers, ticker)
		}
	}

	log.Printf("Có %d active stocks", len(activeTickers))

	// Sort theo tiêu chí
	switch sortBy {
	case SortByVolume:
		sort.Slice(activeTickers, func(i, j int) bool {
			return activeTickers[i].Day.Volume > activeTickers[j].Day.Volume
		})
		log.Printf("Đã sort theo volume")
	case SortByMarketCap:
		sort.Slice(activeTickers, func(i, j int) bool {
			// Nếu không có market cap, dùng volume * close price như proxy
			capI := activeTickers[i].MarketCap
			if capI == 0 {
				capI = float64(activeTickers[i].Day.Volume) * activeTickers[i].Day.Close
			}
			capJ := activeTickers[j].MarketCap
			if capJ == 0 {
				capJ = float64(activeTickers[j].Day.Volume) * activeTickers[j].Day.Close
			}
			return capI > capJ
		})
		log.Printf("Đã sort theo market cap")
	case SortByVWAP:
		sort.Slice(activeTickers, func(i, j int) bool {
			return activeTickers[i].Day.VWAP > activeTickers[j].Day.VWAP
		})
		log.Printf("Đã sort theo VWAP")
	}

	// Lấy top N
	if len(activeTickers) > count {
		activeTickers = activeTickers[:count]
	}

	// Extract ticker symbols
	tickers := make([]string, len(activeTickers))
	for i, t := range activeTickers {
		tickers[i] = t.Ticker
	}

	log.Printf("Đã lấy được top %d stocks theo tiêu chí đã chọn", len(tickers))
	return tickers, nil
}

// GetSP500Stocks lấy danh sách S&P 500 stocks
// Note: Polygon API không có endpoint riêng cho S&P 500, nên ta sẽ lấy top 500 theo market cap
func (c *Crawler) GetSP500Stocks() ([]string, error) {
	// S&P 500 thường là top 500 theo market cap
	return c.GetTopStocks(500, SortByMarketCap)
}

// GetTopStocksByVolume lấy top N stocks theo volume
func (c *Crawler) GetTopStocksByVolume(count int) ([]string, error) {
	return c.GetTopStocks(count, SortByVolume)
}

// GetTopStocksByMarketCap lấy top N stocks theo market cap
func (c *Crawler) GetTopStocksByMarketCap(count int) ([]string, error) {
	return c.GetTopStocks(count, SortByMarketCap)
}
