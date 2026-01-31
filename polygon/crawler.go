package polygon

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"us-data/saver"
)

const (
	// Limit tối đa 50k results mỗi request
	maxLimit = 50000

	// Số ngày tối đa cho mỗi request aggregates 1-minute
	// Tính toán: 50,000 bars / ~960 phút/ngày (extended hours) ≈ 52 ngày
	// Để an toàn, đặt 50 ngày: 50 * 960 = 48,000 < 50,000 (cận tối đa)
	maxDaysPerRequest = 50
)

// AggregatesResponse là response từ Polygon API với next_url
type AggregatesResponse struct {
	Ticker       string   `json:"ticker"`
	QueryCount   int      `json:"queryCount"`
	ResultsCount int      `json:"resultsCount"`
	Adjusted     bool     `json:"adjusted"`
	Results      []BarRaw `json:"results"` // Parse với BarRaw trước, sau đó convert
	Status       string   `json:"status"`
	RequestID    string   `json:"request_id"`
	Count        int      `json:"count"`
	NextURL      string   `json:"next_url,omitempty"`
}

// FlexibleInt64 là type để parse cả int và float (scientific notation) thành int64
type FlexibleInt64 int64

// UnmarshalJSON custom unmarshaler để parse cả int và float
func (f *FlexibleInt64) UnmarshalJSON(data []byte) error {
	// Thử parse như string trước để handle scientific notation
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		// Parse từ string
		val, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return err
		}
		*f = FlexibleInt64(int64(val))
		return nil
	}

	// Thử parse như float64
	var floatVal float64
	if err := json.Unmarshal(data, &floatVal); err == nil {
		*f = FlexibleInt64(int64(floatVal))
		return nil
	}

	// Thử parse như int64
	var intVal int64
	if err := json.Unmarshal(data, &intVal); err == nil {
		*f = FlexibleInt64(intVal)
		return nil
	}

	return fmt.Errorf("không thể parse thành int64: %s", string(data))
}

// Int64 trả về giá trị int64
func (f FlexibleInt64) Int64() int64 {
	return int64(f)
}

// BarRaw là struct tạm để parse JSON với FlexibleInt64 cho Volume và Transactions
type BarRaw struct {
	Timestamp    int64         `json:"t"` // Unix timestamp in milliseconds
	Open         float64       `json:"o"`
	High         float64       `json:"h"`
	Low          float64       `json:"l"`
	Close        float64       `json:"c"`
	Volume       FlexibleInt64 `json:"v"` // Parse với FlexibleInt64 để handle cả int và float
	VWAP         float64       `json:"vw,omitempty"`
	Transactions FlexibleInt64 `json:"n,omitempty"` // Parse với FlexibleInt64
}

// ToBar convert BarRaw sang Bar
func (br BarRaw) ToBar() Bar {
	return Bar{
		Timestamp:    br.Timestamp,
		Open:         br.Open,
		High:         br.High,
		Low:          br.Low,
		Close:        br.Close,
		Volume:       br.Volume.Int64(), // Convert FlexibleInt64 -> int64
		VWAP:         br.VWAP,
		Transactions: br.Transactions.Int64(), // Convert FlexibleInt64 -> int64
	}
}

// TickersResponse là response từ Tickers API với next_url
type TickersResponse struct {
	Status    string   `json:"status"`
	RequestID string   `json:"request_id,omitempty"`
	Count     int      `json:"count"`
	NextURL   string   `json:"next_url,omitempty"`
	Results   []Ticker `json:"results"`
}

// Ticker đại diện cho một ticker trong danh sách
type Ticker struct {
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
	Market string `json:"market"`
	Active bool   `json:"active"`
}

// RateLimiter interface cho rate limiting
type RateLimiter interface {
	WaitForKey(apiKey string) error
	Wait() error
	Close() error
	SaveNextURL(ticker string, nextURL string) error
	GetNextURL(ticker string) (string, error)
	DeleteNextURL(ticker string) error
}

// RateLimitedTransport là custom HTTP transport để tích hợp rate limiter
type RateLimitedTransport struct {
	baseTransport http.RoundTripper
	rateLimiter   RateLimiter
	getAPIKey     func() (string, error)
}

// RoundTrip thực hiện request với rate limiting
func (t *RateLimitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Lấy API key
	apiKey, err := t.getAPIKey()
	if err != nil {
		return nil, fmt.Errorf("lỗi khi lấy API key: %w", err)
	}

	// Đợi rate limiter TRƯỚC KHI gọi API
	if err := t.rateLimiter.WaitForKey(apiKey); err != nil {
		return nil, fmt.Errorf("lỗi khi đợi rate limiter: %w", err)
	}

	// Set Connection: close để kill connection reuse và tránh timeout
	// Có thể connection reuse gây ra vấn đề timeout từ chunk 2 trở đi
	req.Header.Set("Connection", "close")

	// Forward request đến base transport
	resp, err := t.baseTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Crawler quản lý việc crawl dữ liệu từ Massive API
type Crawler struct {
	apiKey        string      // API key hiện tại (cho compatibility)
	keyPool       *APIKeyPool // Pool của nhiều API keys
	rateLimiter   RateLimiter // Rate limiter interface
	httpClient    *http.Client
	useKeyPool    bool              // Flag để biết có dùng key pool không
	SavePacketDir string            // Thư mục lưu packet: {SavePacketDir}/{ticker}/{ticker}_{from}_to_{to}.{ext}
	PacketSaver   saver.PacketSaver // Inject từ ngoài — DIP. Nếu nil thì không lưu packet.
}

// NewCrawler tạo một crawler mới với in-memory rate limiting (single API key)
func NewCrawler(apiKey string) (*Crawler, error) {
	// Tạo in-memory rate limiter
	rateLimiter := NewInMemoryRateLimiter()

	// Tạo custom HTTP transport với timeout dài và rate limiter
	// SDK (RESTY) có thể có timeout riêng, nên cần transport với timeout dài
	// Disable connection reuse để tránh timeout từ chunk 2 trở đi
	baseTransport := &http.Transport{
		ResponseHeaderTimeout: 10 * time.Minute, // Timeout cho response headers
		IdleConnTimeout:       0,                // Disable connection reuse (0 = không reuse)
		TLSHandshakeTimeout:   10 * time.Second,
		DisableKeepAlives:     true, // Disable keep-alive để force close connection
		MaxIdleConns:          0,    // Không giữ idle connections
		MaxIdleConnsPerHost:   0,    // Không giữ idle connections per host
	}

	transport := &RateLimitedTransport{
		baseTransport: baseTransport,
		rateLimiter:   rateLimiter,
		getAPIKey: func() (string, error) {
			return apiKey, nil
		},
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Minute, // Tăng timeout lên 10 phút cho request lớn (50k bars)
	}

	return &Crawler{
		apiKey:      apiKey,
		rateLimiter: rateLimiter,
		httpClient:  httpClient,
		useKeyPool:  false,
	}, nil
}

// NewCrawlerWithKeyPool tạo crawler mới với nhiều API keys
func NewCrawlerWithKeyPool(apiKeys []string, strategy KeySelectionStrategy) (*Crawler, error) {
	if len(apiKeys) == 0 {
		return nil, fmt.Errorf("cần ít nhất một API key")
	}

	// Tạo key pool với in-memory rate limiter
	keyPool, err := NewAPIKeyPool(apiKeys, strategy)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi tạo key pool: %w", err)
	}

	rateLimiter := keyPool.GetRateLimiter()

	// Tạo custom HTTP transport với timeout dài và rate limiter
	// SDK (RESTY) có thể có timeout riêng, nên cần transport với timeout dài
	// Disable connection reuse để tránh timeout từ chunk 2 trở đi
	baseTransport := &http.Transport{
		ResponseHeaderTimeout: 10 * time.Minute, // Timeout cho response headers
		IdleConnTimeout:       0,                // Disable connection reuse (0 = không reuse)
		TLSHandshakeTimeout:   10 * time.Second,
		DisableKeepAlives:     true, // Disable keep-alive để force close connection
		MaxIdleConns:          0,    // Không giữ idle connections
		MaxIdleConnsPerHost:   0,    // Không giữ idle connections per host
	}

	transport := &RateLimitedTransport{
		baseTransport: baseTransport,
		rateLimiter:   rateLimiter,
		getAPIKey: func() (string, error) {
			keyInfo, err := keyPool.GetAvailableKey()
			if err != nil {
				return "", err
			}
			return keyInfo.Key, nil
		},
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Minute, // Tăng timeout lên 10 phút cho request lớn (50k bars)
	}

	return &Crawler{
		apiKey:      apiKeys[0], // Key đầu tiên làm default
		keyPool:     keyPool,
		rateLimiter: rateLimiter,
		httpClient:  httpClient,
		useKeyPool:  true,
	}, nil
}

// getAPIKey lấy API key để sử dụng (từ pool nếu có, không thì dùng default)
func (c *Crawler) getAPIKey() (string, error) {
	if c.useKeyPool && c.keyPool != nil {
		keyInfo, err := c.keyPool.GetAvailableKey()
		if err != nil {
			return "", err
		}
		return keyInfo.Key, nil
	}
	return c.apiKey, nil
}

// GetKeyPoolStats trả về thống kê của key pool (nếu có)
func (c *Crawler) GetKeyPoolStats() map[string]interface{} {
	if c.useKeyPool && c.keyPool != nil {
		return c.keyPool.GetStats()
	}
	return nil
}

// Close đóng các kết nối
func (c *Crawler) Close() error {
	if c.useKeyPool && c.keyPool != nil {
		return c.keyPool.Close()
	}
	if c.rateLimiter != nil {
		return c.rateLimiter.Close()
	}
	return nil
}

// Bar đại diện cho một bar/candle trong dữ liệu aggregates
type Bar struct {
	Timestamp    int64   `json:"t" parquet:"t"`                      // Unix timestamp in milliseconds
	Open         float64 `json:"o" parquet:"o"`                      // Open
	High         float64 `json:"h" parquet:"h"`                      // High
	Low          float64 `json:"l" parquet:"l"`                      // Low
	Close        float64 `json:"c" parquet:"c"`                      // Close
	Volume       int64   `json:"v" parquet:"v"`                      // Volume
	VWAP         float64 `json:"vw,omitempty" parquet:"vw,optional"` // Volume weighted average price
	Transactions int64   `json:"n,omitempty" parquet:"n,optional"`   // Number of transactions
}

// splitDateRangeIntoChunks chia khoảng thời gian [from, to] thành các chunk theo ngày
// để đảm bảo mỗi request không trả về quá ~maxLimit bars (với 1-minute bars).
func splitDateRangeIntoChunks(from, to time.Time, maxDays int) [][2]time.Time {
	var chunks [][2]time.Time

	// Chuẩn hóa về UTC để tránh lệch do timezone
	start := from.UTC()
	end := to.UTC()

	if !start.Before(end) && !start.Equal(end) {
		return chunks
	}

	for currentStart := start; !currentStart.After(end); {
		// Mỗi chunk tối đa maxDays ngày, inclusive
		currentEnd := currentStart.AddDate(0, 0, maxDays-1)
		if currentEnd.After(end) {
			currentEnd = end
		}

		chunks = append(chunks, [2]time.Time{currentStart, currentEnd})

		// Nếu đã tới cuối khoảng thời gian thì dừng
		if currentEnd.Equal(end) {
			break
		}

		// Chunk tiếp theo bắt đầu từ ngày kế tiếp
		currentStart = currentEnd.AddDate(0, 0, 1)
	}

	return chunks
}

// fetchWithNextURL gọi API với URL và xử lý next_url
// QUAN TRỌNG: Hàm này PHẢI đợi rate limiter trước khi gọi API
func (c *Crawler) fetchWithNextURL(requestURL string) (*AggregatesResponse, error) {
	// Lấy API key (nếu dùng key pool, GetAvailableKey() đã đợi rate limiter rồi)
	apiKey, err := c.getAPIKey()
	if err != nil {
		return nil, fmt.Errorf("lỗi khi lấy API key: %w", err)
	}

	// QUAN TRỌNG: Đợi rate limiter TRƯỚC KHI gọi API
	// Nếu dùng key pool, GetAvailableKey() đã đợi rồi
	// Nhưng nếu dùng single key, phải đợi ở đây
	if !c.useKeyPool {
		if err := c.rateLimiter.WaitForKey(apiKey); err != nil {
			return nil, fmt.Errorf("lỗi khi đợi rate limiter: %w", err)
		}
	}
	// Nếu dùng key pool, GetAvailableKey() đã gọi WaitForKey rồi, không cần đợi lại

	// Đảm bảo URL có API key
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi parse URL: %w", err)
	}

	// Thêm API key nếu chưa có hoặc thay thế nếu đã có
	if parsedURL.Query().Get("apiKey") == "" {
		if parsedURL.RawQuery == "" {
			parsedURL.RawQuery = "apiKey=" + apiKey
		} else {
			parsedURL.RawQuery += "&apiKey=" + apiKey
		}
	} else {
		// Thay thế API key cũ bằng key mới từ pool
		query := parsedURL.Query()
		query.Set("apiKey", apiKey)
		parsedURL.RawQuery = query.Encode()
	}
	requestURL = parsedURL.String()

	log.Printf("Đang gọi API: %s", requestURL)

	// Thực hiện request với retry logic cho rate limit
	maxRetries := 3
	retryDelay := 15 * time.Second

	var resp *http.Response

	for attempt := 1; attempt <= maxRetries; attempt++ {
		var err error
		resp, err = c.httpClient.Get(requestURL)
		if err != nil {
			if attempt < maxRetries {
				log.Printf("Lỗi network (attempt %d/%d), retry sau %v...", attempt, maxRetries, retryDelay)
				time.Sleep(retryDelay)
				continue
			}
			return nil, fmt.Errorf("lỗi khi gọi API sau %d attempts: %w", maxRetries, err)
		}

		// Kiểm tra status code
		if resp.StatusCode == http.StatusOK {
			break // Thành công
		}

		// Đọc body để check error message
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Nếu là rate limit (429), retry sau khi sleep
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 429 {
			if attempt < maxRetries {
				log.Printf("Rate limit exceeded (429) - attempt %d/%d, đợi %v trước khi retry...", attempt, maxRetries, retryDelay)
				// Sleep 15 giây để đợi rate limit reset
				time.Sleep(retryDelay)
				// Đợi thêm rate limiter trước khi retry (không tính request này vào rate limit)
				if err := c.rateLimiter.WaitForKey(apiKey); err != nil {
					log.Printf("Cảnh báo: không thể đợi rate limiter: %v", err)
				}
				continue
			}
			return nil, fmt.Errorf("API trả về rate limit (429) sau %d attempts: %s", maxRetries, string(body))
		}

		// Các lỗi khác không retry
		return nil, fmt.Errorf("API trả về status %d: %s", resp.StatusCode, string(body))
	}
	defer resp.Body.Close()

	// Parse JSON response
	var result AggregatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("lỗi khi parse JSON: %w", err)
	}

	// Kiểm tra status trong response
	if result.Status != "OK" {
		return nil, fmt.Errorf("API trả về status không OK: %s", result.Status)
	}

	return &result, nil
}

// CrawlMinuteBarsWithNextURL crawl dữ liệu minute bars gọi API trực tiếp (không dùng SDK).
// Bao gồm cả extended hours (pre-market 4:00-9:30 ET và after-hours 16:00-20:00 ET).
// ĐÃ CHIA NHỎ date range thành nhiều chunk theo ngày để không vượt quá limit 50k results/request.
func (c *Crawler) CrawlMinuteBarsWithNextURL(ticker string, from, to time.Time) ([]Bar, error) {
	var allBars []Bar

	// Chia khoảng thời gian thành các chunk theo ngày
	chunks := splitDateRangeIntoChunks(from, to, maxDaysPerRequest)
	if len(chunks) == 0 {
		log.Printf("[%s] Không có chunk nào trong khoảng thời gian từ %s đến %s", ticker, from, to)
		return allBars, nil
	}

	log.Printf("[%s] Chia khoảng thời gian thành %d chunks (tối đa %d ngày/chunk)", ticker, len(chunks), maxDaysPerRequest)

	chunkCount := 0

	for chunkIndex, ch := range chunks {
		chunkFrom := ch[0]
		chunkTo := ch[1]

		// Nếu chunk cuối cùng và chunkTo là ngày hiện tại, trừ đi 1 ngày để tránh lỗi DELAYED
		if chunkIndex == len(chunks)-1 {
			now := time.Now().UTC()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			chunkToDate := time.Date(chunkTo.Year(), chunkTo.Month(), chunkTo.Day(), 0, 0, 0, 0, time.UTC)
			if chunkToDate.Equal(today) || chunkToDate.After(today) {
				chunkTo = today.AddDate(0, 0, -1).Add(23*time.Hour + 59*time.Minute + 59*time.Second)
				log.Printf("[%s] Chunk cuối cùng: Trừ 1 ngày để tránh lỗi DELAYED (từ %s)", ticker, chunkToDate.Format("2006-01-02"))
			}
		}

		fromStr := chunkFrom.Format("2006-01-02")
		toStr := chunkTo.Format("2006-01-02")

		log.Printf("[%s] Chunk %d/%d: Crawling từ %s đến %s", ticker, chunkIndex+1, len(chunks), fromStr, toStr)

		// Build URL với timestamp milliseconds
		fromMillis := chunkFrom.UnixMilli()
		toMillis := chunkTo.UnixMilli()

		// Gọi API trực tiếp
		apiKey, err := c.getAPIKey()
		if err != nil {
			return nil, fmt.Errorf("lỗi khi lấy API key: %w", err)
		}

		// Build URL: https://api.massive.com/v2/aggs/ticker/{ticker}/range/{multiplier}/{timespan}/{from}/{to}
		requestURL := fmt.Sprintf("https://api.massive.com/v2/aggs/ticker/%s/range/1/minute/%d/%d", ticker, fromMillis, toMillis)
		u, err := url.Parse(requestURL)
		if err != nil {
			return nil, fmt.Errorf("lỗi khi parse URL: %w", err)
		}

		q := u.Query()
		q.Set("adjusted", "true")
		q.Set("limit", strconv.Itoa(maxLimit))
		q.Set("sort", "asc")
		q.Set("apiKey", apiKey)
		u.RawQuery = q.Encode()

		// Rate limiter đã đợi trong RoundTrip khi gọi httpClient.Get
		// Gọi API với retry logic
		var response *AggregatesResponse
		maxRetries := 3
		retryDelay := 15 * time.Second

		for attempt := 1; attempt <= maxRetries; attempt++ {
			req, err := http.NewRequest("GET", u.String(), nil)
			if err != nil {
				return nil, fmt.Errorf("lỗi khi tạo request: %w", err)
			}
			req.Header.Set("Connection", "close")

			resp, err := c.httpClient.Do(req)
			if err != nil {
				if attempt < maxRetries {
					log.Printf("[%s] Chunk %d/%d - Lỗi network (attempt %d/%d), retry sau %v...",
						ticker, chunkIndex+1, len(chunks), attempt, maxRetries, retryDelay)
					time.Sleep(retryDelay)
					continue
				}
				return nil, fmt.Errorf("lỗi khi gọi API sau %d attempts: %w", maxRetries, err)
			}

			// Kiểm tra status code
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 429 {
					if attempt < maxRetries {
						log.Printf("[%s] Chunk %d/%d - Rate limit (429) - attempt %d/%d, đợi %v trước khi retry...",
							ticker, chunkIndex+1, len(chunks), attempt, maxRetries, retryDelay)
						time.Sleep(retryDelay)
						// Đợi thêm rate limiter trước khi retry
						if err := c.rateLimiter.WaitForKey(apiKey); err != nil {
							log.Printf("Cảnh báo: không thể đợi rate limiter: %v", err)
						}
						continue
					}
					return nil, fmt.Errorf("API trả về rate limit (429) sau %d attempts: %s", maxRetries, string(body))
				}

				resp.Body.Close()
				return nil, fmt.Errorf("API trả về status %d: %s", resp.StatusCode, string(body))
			}

			// Parse JSON response
			var result AggregatesResponse
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				resp.Body.Close()
				if attempt < maxRetries {
					log.Printf("[%s] Chunk %d/%d - Lỗi parse JSON (attempt %d/%d), retry sau %v...",
						ticker, chunkIndex+1, len(chunks), attempt, maxRetries, retryDelay)
					time.Sleep(retryDelay)
					continue
				}
				return nil, fmt.Errorf("lỗi khi parse JSON: %w", err)
			}
			resp.Body.Close()

			// Kiểm tra status trong response
			if result.Status != "OK" {
				// Nếu là DELAYED (dữ liệu chưa sẵn sàng), skip chunk này và tiếp tục
				if result.Status == "DELAYED" {
					log.Printf("[%s] Chunk %d/%d - Dữ liệu DELAYED (chưa sẵn sàng), skip chunk này",
						ticker, chunkIndex+1, len(chunks))
					response = nil // Đánh dấu là đã skip
					break
				}
				return nil, fmt.Errorf("API trả về status không OK: %s", result.Status)
			}

			response = &result
			break // Thành công
		}

		// Nếu response là nil (do DELAYED hoặc lỗi), skip chunk này
		if response == nil {
			// DELAYED đã được log ở trên, skip chunk này và tiếp tục
			log.Printf("[%s] Chunk %d/%d - Skip do DELAYED hoặc lỗi", ticker, chunkIndex+1, len(chunks))
			continue
		}

		// Convert BarRaw sang Bar
		chunkBars := 0
		for _, barRaw := range response.Results {
			allBars = append(allBars, barRaw.ToBar())
			chunkBars++
		}

		// Nếu có next_url, có nghĩa là chunk này có nhiều hơn 50k bars
		if response.NextURL != "" {
			log.Printf("[%s] Chunk %d/%d - CẢNH BÁO: Có next_url (có thể vượt quá 50k bars)",
				ticker, chunkIndex+1, len(chunks))
		}

		chunkCount++
		log.Printf("[%s] Chunk %d/%d - Hoàn thành: %d bars (tổng: %d)",
			ticker, chunkIndex+1, len(chunks), chunkBars, len(allBars))
	}

	log.Printf("[%s] Hoàn thành: %d chunks, tổng %d bars", ticker, chunkCount, len(allBars))

	// Lưu 1 file duy nhất cho cả ticker (toàn bộ khoảng from–to) nếu SavePacketDir và PacketSaver được set
	if c.SavePacketDir != "" && c.PacketSaver != nil && len(allBars) > 0 {
		tickerDir := filepath.Join(c.SavePacketDir, ticker)
		if err := os.MkdirAll(tickerDir, 0755); err != nil {
			log.Printf("[%s] ⚠️ Save: không tạo được folder %s: %v", ticker, tickerDir, err)
		} else {
			ext := c.PacketSaver.Extension()
			packetName := fmt.Sprintf("%s_%s_to_%s.%s",
				ticker, from.Format("2006-01-02"), to.Format("2006-01-02"), ext)
			packetPath := filepath.Join(tickerDir, packetName)
			saverBars := make([]saver.Bar, len(allBars))
			for i, b := range allBars {
				saverBars[i] = saver.Bar{
					Timestamp: b.Timestamp, Open: b.Open, High: b.High, Low: b.Low,
					Close: b.Close, Volume: b.Volume, VWAP: b.VWAP, Transactions: b.Transactions,
				}
			}
			if err := c.PacketSaver.Save(saverBars, packetPath); err != nil {
				log.Printf("[%s] ⚠️ Save: lỗi lưu %s: %v", ticker, packetPath, err)
			} else {
				log.Printf("[%s] 📦 Saved 1 file (%s): %s (%d bars)", ticker, ext, packetPath, len(allBars))
			}
		}
	}

	return allBars, nil
}

// CrawlMinuteBars crawl dữ liệu minute bars (wrapper để tương thích)
func (c *Crawler) CrawlMinuteBars(ticker string, from, to time.Time) ([]Bar, error) {
	return c.CrawlMinuteBarsWithNextURL(ticker, from, to)
}
