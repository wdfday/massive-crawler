package polygon

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
)

// GetUSTickersWithNextURL lấy danh sách tickers với xử lý next_url thủ công
func (c *Crawler) GetUSTickersWithNextURL(targetCount int) ([]string, error) {
	var allTickers []string
	seenTickers := make(map[string]bool)

	// Kiểm tra xem có next_url đã lưu từ lần trước không
	savedNextURL, err := c.rateLimiter.GetNextURL("tickers")
	if err != nil {
		log.Printf("Cảnh báo: không thể lấy saved next_url: %v", err)
	}

	nextURL := savedNextURL
	if nextURL == "" {
		// Tạo URL request đầu tiên
		nextURL = fmt.Sprintf("%s/v3/reference/tickers?market=stocks&active=true&limit=1000&order=asc",
			polygonBaseURL)
	}

	for len(allTickers) < targetCount && nextURL != "" {
		// Đợi rate limiter và lấy API key
		apiKey, err := c.getAPIKey()
		if err != nil {
			return nil, fmt.Errorf("lỗi khi lấy API key: %w", err)
		}

		// Đảm bảo URL có API key
		parsedURL, err := url.Parse(nextURL)
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
		nextURL = parsedURL.String()

		log.Printf("Đang lấy tickers từ: %s", nextURL)

		// Thực hiện request
		resp, err := c.httpClient.Get(nextURL)
		if err != nil {
			// Lưu next_url để có thể retry sau
			c.rateLimiter.SaveNextURL("tickers", nextURL)
			return nil, fmt.Errorf("lỗi khi gọi API: %w", err)
		}

		// Parse response
		var result TickersResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			c.rateLimiter.SaveNextURL("tickers", nextURL)
			return nil, fmt.Errorf("lỗi khi parse JSON: %w", err)
		}
		resp.Body.Close()

		// Kiểm tra status
		if result.Status != "OK" {
			return nil, fmt.Errorf("API trả về status không OK: %s", result.Status)
		}

		// Thêm tickers vào danh sách (tránh duplicate)
		batchCount := 0
		for _, ticker := range result.Results {
			if ticker.Market == "stocks" && ticker.Active && !seenTickers[ticker.Ticker] {
				allTickers = append(allTickers, ticker.Ticker)
				seenTickers[ticker.Ticker] = true
				batchCount++

				if len(allTickers) >= targetCount {
					break
				}
			}
		}

		log.Printf("Đã lấy được %d tickers trong batch này (tổng: %d)", batchCount, len(allTickers))

		// Kiểm tra next_url
		if result.NextURL != "" && len(allTickers) < targetCount {
			nextURL = result.NextURL
			// Note: next_url được handle tự động bởi SDK
			if err := c.rateLimiter.SaveNextURL("tickers", nextURL); err != nil {
				log.Printf("Cảnh báo: không thể lưu next_url: %v", err)
			}
		} else {
			// Không còn next_url hoặc đã đủ số lượng
			nextURL = ""
			// Xóa next_url đã lưu vì đã xong
			c.rateLimiter.DeleteNextURL("tickers")
		}
	}

	log.Printf("Tổng cộng đã lấy được %d tickers US stocks", len(allTickers))

	// Giới hạn về số lượng yêu cầu
	if len(allTickers) > targetCount {
		allTickers = allTickers[:targetCount]
	}

	return allTickers, nil
}

// GetUSTickers lấy danh sách tickers (wrapper để tương thích)
func (c *Crawler) GetUSTickers(limit int) ([]string, error) {
	return c.GetUSTickersWithNextURL(limit)
}

// GetUSTickersWithPagination wrapper để tương thích
func (c *Crawler) GetUSTickersWithPagination(targetCount int) ([]string, error) {
	return c.GetUSTickersWithNextURL(targetCount)
}
