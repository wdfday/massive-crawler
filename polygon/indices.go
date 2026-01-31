package polygon

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// LoadTickersFromFile đọc danh sách tickers từ file
// Hỗ trợ các format: .txt (một ticker mỗi dòng), .json (array of strings)
func LoadTickersFromFile(filepath string) ([]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("không thể mở file %s: %w", filepath, err)
	}
	defer file.Close()

	// Đọc toàn bộ nội dung
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("không thể đọc file: %w", err)
	}

	var tickers []string

	// Kiểm tra extension để xác định format
	ext := strings.ToLower(filepath[strings.LastIndex(filepath, "."):])

	switch ext {
	case ".json":
		// Parse JSON
		if err := json.Unmarshal(content, &tickers); err != nil {
			return nil, fmt.Errorf("lỗi khi parse JSON: %w", err)
		}
	case ".txt":
		// Parse text file (một ticker mỗi dòng)
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				tickers = append(tickers, line)
			}
		}
	default:
		// Thử parse như JSON trước, nếu fail thì parse như text
		if err := json.Unmarshal(content, &tickers); err != nil {
			// Parse như text file
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					tickers = append(tickers, line)
				}
			}
		}
	}

	// Loại bỏ empty và duplicate
	seen := make(map[string]bool)
	var uniqueTickers []string
	for _, ticker := range tickers {
		ticker = strings.TrimSpace(strings.ToUpper(ticker))
		if ticker != "" && !seen[ticker] {
			seen[ticker] = true
			uniqueTickers = append(uniqueTickers, ticker)
		}
	}

	log.Printf("Đã load %d tickers từ file %s", len(uniqueTickers), filepath)
	return uniqueTickers, nil
}

// LoadTickersFromIndicesFile đọc từ file indices/combined.txt hoặc tickers.json
func LoadTickersFromIndicesFile() ([]string, error) {
	// Thử các đường dẫn có thể
	possiblePaths := []string{
		"indices/combined.txt",
		"indices/tickers.json",
		"indices/sp500.txt",
		"scripts/../indices/combined.txt",
		"scripts/../indices/tickers.json",
	}

	for _, path := range possiblePaths {
		// Resolve relative path
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}

		if _, err := os.Stat(absPath); err == nil {
			log.Printf("Tìm thấy file indices tại: %s", absPath)
			return LoadTickersFromFile(absPath)
		}
	}

	return nil, fmt.Errorf("không tìm thấy file indices. Vui lòng chạy script fetch_indices.sh hoặc fetch_indices.py trước")
}

// LoadTickersFromFileOrIndices thử load từ file chỉ định, nếu không có thì load từ indices
func LoadTickersFromFileOrIndices(filepath string) ([]string, error) {
	if filepath != "" {
		// Thử load từ file chỉ định
		if _, err := os.Stat(filepath); err == nil {
			return LoadTickersFromFile(filepath)
		}
		log.Printf("File %s không tồn tại, thử load từ indices...", filepath)
	}

	// Fallback về indices
	return LoadTickersFromIndicesFile()
}
