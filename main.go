package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"us-data/polygon"
	"us-data/provider"
	"us-data/saver"
)

func main() {
	// Đọc data provider từ environment variable (mặc định: polygon)
	dataProviderName := os.Getenv("DATA_PROVIDER")
	if dataProviderName == "" {
		dataProviderName = "polygon"
	}

	var dataProvider provider.DataProvider
	var err error

	// Tạo provider dựa trên config
	switch strings.ToLower(dataProviderName) {
	case "polygon":
		dataProvider, err = createPolygonProvider()
	case "tiingo":
		dataProvider, err = createTiingoProvider()
	default:
		log.Fatalf("Không hỗ trợ data provider: %s. Options: polygon, tiingo", dataProviderName)
	}

	if err != nil {
		log.Fatalf("Lỗi khi tạo data provider: %v", err)
	}
	defer dataProvider.Close()

	log.Printf("Sử dụng data provider: %s", dataProvider.GetName())

	// Đọc cách lấy stocks từ environment variable
	// Options: "file", "top-marketcap", "top-volume", "sp500", hoặc "any"
	stockSelection := os.Getenv("STOCK_SELECTION")
	if stockSelection == "" {
		stockSelection = "file" // Mặc định đọc từ file indices
	}

	// Đọc đường dẫn file từ environment variable (nếu có)
	tickersFilePath := os.Getenv("TICKERS_FILE")

	var tickers []string

	// Lấy danh sách tickers theo cách đã chọn
	switch stockSelection {
	case "file":
		// Đọc từ file (S&P 500 + NASDAQ 100)
		log.Println("Đang đọc danh sách tickers từ file...")
		if tickersFilePath != "" {
			tickers, err = polygon.LoadTickersFromFileOrIndices(tickersFilePath)
		} else {
			tickers, err = polygon.LoadTickersFromIndicesFile()
		}
		if err != nil {
			log.Fatalf("Lỗi khi đọc file tickers: %v\nHãy chạy: bash scripts/fetch_indices.sh hoặc python3 scripts/fetch_indices.py", err)
		}
	case "top-marketcap":
		log.Println("Đang lấy top 500 mã US stocks theo market cap...")
		// Cần polygon crawler để lấy top stocks
		polygonCrawler, err := createPolygonCrawler()
		if err != nil {
			log.Fatalf("Lỗi khi tạo polygon crawler: %v", err)
		}
		defer polygonCrawler.Close()
		tickers, err = polygonCrawler.GetTopStocksByMarketCap(500)
	case "top-volume":
		log.Println("Đang lấy top 500 mã US stocks theo volume...")
		polygonCrawler, err := createPolygonCrawler()
		if err != nil {
			log.Fatalf("Lỗi khi tạo polygon crawler: %v", err)
		}
		defer polygonCrawler.Close()
		tickers, err = polygonCrawler.GetTopStocksByVolume(500)
	case "sp500":
		log.Println("Đang lấy S&P 500 stocks (top 500 theo market cap)...")
		polygonCrawler, err := createPolygonCrawler()
		if err != nil {
			log.Fatalf("Lỗi khi tạo polygon crawler: %v", err)
		}
		defer polygonCrawler.Close()
		tickers, err = polygonCrawler.GetSP500Stocks()
	case "any":
		log.Println("Đang lấy danh sách 500 mã US stocks (bất kỳ)...")
		polygonCrawler, err := createPolygonCrawler()
		if err != nil {
			log.Fatalf("Lỗi khi tạo polygon crawler: %v", err)
		}
		defer polygonCrawler.Close()
		tickers, err = polygonCrawler.GetUSTickersWithPagination(500)
	default:
		log.Printf("Không nhận diện được STOCK_SELECTION='%s', sử dụng file", stockSelection)
		tickers, err = polygon.LoadTickersFromIndicesFile()
		if err != nil {
			log.Fatalf("Lỗi khi đọc file tickers: %v\nHãy chạy: bash scripts/fetch_indices.sh hoặc python3 scripts/fetch_indices.py", err)
		}
	}

	if err != nil {
		log.Fatalf("Lỗi khi lấy danh sách tickers: %v", err)
	}

	log.Printf("Đã lấy được %d tickers", len(tickers))

	// Lưu danh sách tickers vào file
	tickersFile := "tickers.json"
	if err = saveTickersToFile(tickers, tickersFile); err != nil {
		log.Printf("Cảnh báo: không thể lưu danh sách tickers: %v", err)
	}

	// Crawl từ (now - 2 năm) đến (now + 1 ngày) để bao gồm trọn ngày hôm nay
	now := time.Now().UTC()
	fromDate := time.Date(now.Year()-2, now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	toDate := now.AddDate(0, 0, 1)

	log.Printf("Bắt đầu crawl minute bars từ %s đến %s cho %d tickers",
		fromDate.Format("2006-01-02"), toDate.Format("2006-01-02"), len(tickers))

	// Đọc thư mục lưu dữ liệu từ environment variable (mặc định: data)
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}

	// Tạo thư mục để lưu dữ liệu
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Lỗi khi tạo thư mục data: %v", err)
	}

	// Đường dẫn lưu packet: data/Polygon/{Ticker}/
	saveBaseDir := filepath.Join(dataDir, "Polygon")
	saveFormat := getSaveFormat() // SAVE_FORMAT hoặc mặc định theo PROFILE

	log.Printf("📂 [SAVE DIR] Thư mục: %s", saveBaseDir)
	log.Printf("📂 [SAVE DIR] Định dạng packet: %s (SAVE_FORMAT hoặc PROFILE)", saveFormat)
	log.Printf("📂 [SAVE DIR] Cấu trúc: %s/{Ticker}/{ticker}_{from}_to_{to}.%s", saveBaseDir, saveFormat)

	// --- Wire: Polygon + PacketSaver (DIP: inject saver vào crawler) ---
	wirePolygonPacketSave(dataProvider, saveBaseDir, saveFormat)

	// Crawl dữ liệu cho từng ticker - TUẦN TỰ (một mã tại một thời điểm)
	// Mã nào xong hoàn toàn (crawl + lưu file) mới chuyển sang mã tiếp theo
	successCount := 0
	failedCount := 0

	for i, ticker := range tickers {
		log.Printf("\n[%d/%d] Bắt đầu crawl %s...", i+1, len(tickers), ticker)

		// Crawl dữ liệu cho ticker này (chờ hoàn thành)
		bars, err := dataProvider.CrawlMinuteBars(ticker, fromDate, toDate)
		if err != nil {
			log.Printf("[%s] ❌ Lỗi khi crawl: %v", ticker, err)
			failedCount++
			continue // Chuyển sang ticker tiếp theo
		}

		if len(bars) == 0 {
			log.Printf("[%s] ⚠️  Không có dữ liệu", ticker)
			failedCount++
			continue // Chuyển sang ticker tiếp theo
		}

		// Packet đã được lưu trong crawler (mỗi chunk 1 file theo SAVE_FORMAT)
		log.Printf("[%s] ✅ Hoàn thành! %d bars (đã lưu packet trong crawl)", ticker, len(bars))
		successCount++
	}

	log.Printf("\n=== Kết quả ===")
	log.Printf("Thành công: %d/%d tickers", successCount, len(tickers))
	log.Printf("Thất bại: %d/%d tickers", failedCount, len(tickers))
	log.Printf("Data provider: %s", dataProvider.GetName())
	log.Println("Hoàn thành!")

	// Lưu ý: Massive SDK tự động handle pagination với next_url
}

// getSaveFormat trả về định dạng lưu packet: SAVE_FORMAT, hoặc mặc định theo PROFILE (dev→json, prod/empty→parquet).
func getSaveFormat() string {
	if v := os.Getenv("SAVE_FORMAT"); v != "" {
		return v
	}
	switch os.Getenv("PROFILE") {
	case "dev", "development":
		return "json"
	case "prod", "production", "":
		return "parquet"
	default:
		return "parquet"
	}
}

// wirePolygonPacketSave wire PacketSaver vào Polygon provider (DIP). Chỉ gọi khi provider là Polygon.
func wirePolygonPacketSave(dp provider.DataProvider, saveBaseDir, saveFormat string) {
	p, ok := dp.(*provider.PolygonProvider)
	if !ok {
		return
	}
	packetSaver := saver.NewPacketSaver(saveFormat)
	if packetSaver == nil {
		log.Fatalf("SAVE_FORMAT=%q không hợp lệ. Dùng: csv, parquet, json", saveFormat)
	}
	p.SetSavePacketDir(saveBaseDir)
	p.SetPacketSaver(packetSaver)
	log.Printf("📦 [WIRE] Polygon + PacketSaver (%s): %s/{Ticker}/{ticker}_{from}_to_{to}.%s", saveFormat, saveBaseDir, packetSaver.Extension())
}

// createPolygonProvider tạo Polygon provider từ environment variables
func createPolygonProvider() (provider.DataProvider, error) {
	// Đọc API keys từ environment variable
	apiKeysStr := os.Getenv("POLYGON_API_KEYS")
	if apiKeysStr == "" {
		apiKeysStr = os.Getenv("POLYGON_API_KEY")
		if apiKeysStr == "" {
			return nil, fmt.Errorf("POLYGON_API_KEY hoặc POLYGON_API_KEYS không được set")
		}
	}

	// Parse API keys
	apiKeys := strings.Split(apiKeysStr, ",")
	for i := range apiKeys {
		apiKeys[i] = strings.TrimSpace(apiKeys[i])
	}

	// Đọc strategy
	strategyStr := os.Getenv("KEY_STRATEGY")
	var strategy polygon.KeySelectionStrategy = polygon.RoundRobin
	if strategyStr == "least-used" {
		strategy = polygon.LeastUsed
	}

	return provider.NewPolygonProvider(apiKeys, strategy)
}

// createPolygonCrawler tạo Polygon crawler (cho các tính năng đặc biệt như top stocks)
func createPolygonCrawler() (*polygon.Crawler, error) {
	apiKeysStr := os.Getenv("POLYGON_API_KEYS")
	if apiKeysStr == "" {
		apiKeysStr = os.Getenv("POLYGON_API_KEY")
		if apiKeysStr == "" {
			return nil, fmt.Errorf("POLYGON_API_KEY hoặc POLYGON_API_KEYS không được set")
		}
	}

	apiKeys := strings.Split(apiKeysStr, ",")
	for i := range apiKeys {
		apiKeys[i] = strings.TrimSpace(apiKeys[i])
	}

	strategyStr := os.Getenv("KEY_STRATEGY")
	var strategy polygon.KeySelectionStrategy = polygon.RoundRobin
	if strategyStr == "least-used" {
		strategy = polygon.LeastUsed
	}

	if len(apiKeys) == 1 {
		return polygon.NewCrawler(apiKeys[0])
	}
	return polygon.NewCrawlerWithKeyPool(apiKeys, strategy)
}

// createTiingoProvider tạo Tiingo provider từ environment variables
func createTiingoProvider() (provider.DataProvider, error) {
	apiKey := os.Getenv("TIINGO_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("TIINGO_API_KEY không được set")
	}

	return provider.NewTiingoProvider(apiKey)
}

// saveTickersToFile lưu danh sách tickers vào file JSON
func saveTickersToFile(tickers []string, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("lỗi khi tạo file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(tickers); err != nil {
		return fmt.Errorf("lỗi khi ghi JSON: %w", err)
	}

	return nil
}
