package polygon

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"us-data/internal/model"
	"us-data/internal/saver"
)

const (
	// Max 50k results per request
	maxLimit = 50000

	// Max days per 1-minute aggregates request (~50k bars / ~960 min/day ≈ 52 days; use 50 for safety)
	maxDaysPerRequest = 50

	// KeyCooldownSec: Polygon 5 req/min => 12s between requests per key
	KeyCooldownSec = 12

	// Minutes per trading day (max, extended hours)
	minPerDay = 960
)

// estimatedBars returns pre-alloc capacity for [from, to]. days * 960 + 10% buffer. No grow.
func estimatedBars(from, to time.Time) int {
	if !from.Before(to) && !from.Equal(to) {
		return 0
	}
	days := int(to.Sub(from).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	n := days * minPerDay
	// +10% buffer so we never realloc
	n = n + n/10
	if n > 500000 {
		n = 500000 // 504 days * 960 min/day
	}
	return n
}

// LogFunc emits a log line. When set, used instead of log.Printf (fan-in logger).
type LogFunc func(msg string)

// Crawler is responsible for fetching minute-bar aggregates from the Polygon API
// and optionally persisting raw packets to disk.
type Crawler struct {
	client        *http.Client
	SavePacketDir string
	PacketSaver   saver.PacketSaver // When non-nil, used to persist raw packets.
	SavePerDay    bool              // When true, saves one file per day {ticker}_{date}.ext; otherwise a single range file.
	LogFunc       LogFunc           // Optional fan-in logger for crawl progress and diagnostics.
}

func (c *Crawler) logf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if c.LogFunc != nil {
		c.LogFunc(msg)
	} else {
		slog.Info(msg)
	}
}

// Close closes connections
func (c *Crawler) Close() error {
	return nil
}

// saveBarsPacket writes bars to SavePacketDir using PacketSaver if configured.
func (c *Crawler) saveBarsPacket(ticker string, from, to time.Time, bars []model.Bar) {
	if c.SavePacketDir == "" || c.PacketSaver == nil || len(bars) == 0 {
		return
	}
	tickerDir := filepath.Join(c.SavePacketDir, ticker)
	if err := os.MkdirAll(tickerDir, 0755); err != nil {
		c.logf("[%s] Save: cannot create folder %s: %v", ticker, tickerDir, err)
		return
	}
	ext := c.PacketSaver.Extension()
	var packetName string
	if c.SavePerDay {
		packetName = fmt.Sprintf("%s_%s.%s", ticker, from.Format("2006-01-02"), ext)
	} else {
		packetName = fmt.Sprintf("%s_%s_to_%s.%s", ticker, from.Format("2006-01-02"), to.Format("2006-01-02"), ext)
	}
	packetPath := filepath.Join(tickerDir, packetName)
	if err := c.PacketSaver.Save(bars, packetPath); err != nil {
		c.logf("[%s] Save: failed to write %s: %v", ticker, packetPath, err)
	} else {
		c.logf("[%s] Saved 1 file (%s): %s (%d bars)", ticker, ext, packetPath, len(bars))
	}
}

// splitDateRangeIntoChunks splits [from, to] into day chunks so each request stays under ~maxLimit bars
func splitDateRangeIntoChunks(from, to time.Time, maxDays int) [][2]time.Time {
	var chunks [][2]time.Time
	start := from.UTC()
	end := to.UTC()

	if !start.Before(end) && !start.Equal(end) {
		return chunks
	}

	for currentStart := start; !currentStart.After(end); {
		currentEnd := currentStart.AddDate(0, 0, maxDays-1)
		if currentEnd.After(end) {
			currentEnd = end
		}

		chunks = append(chunks, [2]time.Time{currentStart, currentEnd})

		if currentEnd.Equal(end) {
			break
		}

		currentStart = currentEnd.AddDate(0, 0, 1)
	}

	return chunks
}

// adjustLastChunkToAvoidDelayed returns chunkTo unchanged, or end of previous day if chunkTo is today/future (avoids DELAYED).
func adjustLastChunkToAvoidDelayed(chunkTo time.Time, isLastChunk bool) time.Time {
	if !isLastChunk {
		return chunkTo
	}
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	chunkToDate := time.Date(chunkTo.Year(), chunkTo.Month(), chunkTo.Day(), 0, 0, 0, 0, time.UTC)
	if chunkToDate.Equal(today) || chunkToDate.After(today) {
		return today.AddDate(0, 0, -1).Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	}
	return chunkTo
}

const maxRetries = 3
const retryDelay = 15 * time.Second

// buildMinuteAggregatesRequest builds GET request for 1-minute aggregates (adjusted, limit, sort, apiKey).
func (c *Crawler) buildMinuteAggregatesRequest(ticker string, fromMillis, toMillis int64, apiKey string) (*http.Request, error) {
	rawURL := fmt.Sprintf("%s/v2/aggs/ticker/%s/range/1/minute/%d/%d", polygonBaseURL, ticker, fromMillis, toMillis)
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	q := u.Query()
	q.Set("adjusted", "true")
	q.Set("limit", strconv.Itoa(maxLimit))
	q.Set("sort", "asc")
	q.Set("apiKey", apiKey)
	u.RawQuery = q.Encode()
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Connection", "close")
	return req, nil
}

// doAggregatesRequest runs one GET request with retries. On 429 calls on429 before retry.
// Returns (nil, nil) when status is DELAYED (caller should skip chunk); (nil, err) on error; (resp, nil) on success.
func (c *Crawler) doAggregatesRequest(client *http.Client, req *http.Request, on429 func()) (*AggregatesResponse, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := client.Do(req)
		if err != nil {
			if attempt < maxRetries {
				time.Sleep(retryDelay)
				continue
			}
			return nil, fmt.Errorf("API call failed after %d attempts: %w", maxRetries, err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 429 {
				if attempt < maxRetries {
					time.Sleep(retryDelay)
					if on429 != nil {
						on429()
					}
					continue
				}
				return nil, fmt.Errorf("API rate limit (429) after %d attempts: %s", maxRetries, string(body))
			}
			return nil, fmt.Errorf("API status %d: %s", resp.StatusCode, string(body))
		}

		var result AggregatesResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			if attempt < maxRetries {
				time.Sleep(retryDelay)
				continue
			}
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		resp.Body.Close()

		if result.Status != "OK" {
			if result.Status == "DELAYED" {
				return nil, nil // caller skips chunk
			}
			return nil, fmt.Errorf("API status not OK: %s", result.Status)
		}
		return &result, nil
	}
	return nil, fmt.Errorf("no response")
}

// CrawlMinuteBarsWithKey fetches minute-bar aggregates for the given ticker and time range
// using the provided API key. Callers are responsible for API-key rotation and rate limiting.
func (c *Crawler) CrawlMinuteBarsWithKey(ticker, apiKey string, from, to time.Time) ([]model.Bar, error) {
	client := c.client
	if client == nil {
		client = http.DefaultClient
	}

	allBars := make([]model.Bar, 0, estimatedBars(from, to))
	chunks := splitDateRangeIntoChunks(from, to, maxDaysPerRequest)
	if len(chunks) == 0 {
		c.logf("[%s] No chunks in date range %s to %s", ticker, from, to)
		return allBars, nil
	}

	keyPrefix := apiKey
	if len(apiKey) > 8 {
		keyPrefix = apiKey[:8]
	}
	c.logf("[%s] Split into %d chunks (key=%s...)", ticker, len(chunks), keyPrefix)

	for chunkIndex, ch := range chunks {
		// Rate limit: wait 12s before each request (except first chunk) for 5 req/min/key
		if chunkIndex > 0 {
			cooldown := KeyCooldownSec * time.Second
			c.logf("[RATE] [%s] chunk %d/%d: cooldown %ds (key=%s...) start", ticker, chunkIndex+1, len(chunks), KeyCooldownSec, keyPrefix)
			start := time.Now()
			time.Sleep(cooldown)
			elapsed := time.Since(start)
			c.logf("[RATE] [%s] chunk %d/%d: cooldown done (waited %.2fs, key ready)", ticker, chunkIndex+1, len(chunks), elapsed.Seconds())
		}

		chunkFrom := ch[0]
		chunkTo := adjustLastChunkToAvoidDelayed(ch[1], chunkIndex == len(chunks)-1)

		fromMillis := chunkFrom.UnixMilli()
		toMillis := chunkTo.UnixMilli()

		req, err := c.buildMinuteAggregatesRequest(ticker, fromMillis, toMillis, apiKey)
		if err != nil {
			return nil, err
		}
		response, err := c.doAggregatesRequest(client, req, nil)
		if err != nil {
			return nil, err
		}
		if response == nil {
			continue // DELAYED
		}

		for _, barRaw := range response.Results {
			allBars = append(allBars, barRaw.ToBar())
		}

		// Cooldown after last chunk before returning key to chan (key ready for next request)
		if chunkIndex < len(chunks)-1 {
			// Not last chunk: no sleep
		} else {
			cooldown := KeyCooldownSec * time.Second
			c.logf("[RATE] [%s] last chunk done: cooldown %ds (key=%s...) before returning key", ticker, KeyCooldownSec, keyPrefix)
			start := time.Now()
			time.Sleep(cooldown)
			c.logf("[RATE] [%s] cooldown done (waited %.2fs), return key", ticker, time.Since(start).Seconds())
		}
	}
	c.saveBarsPacket(ticker, from, to, allBars)
	return allBars, nil
}
