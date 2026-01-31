package polygon

import (
	"log"
	"sync"
	"time"
)

// InMemoryRateLimiter quản lý rate limiting đơn giản với sleep
type InMemoryRateLimiter struct {
	mu          sync.Mutex
	lastRequest map[string]time.Time // map[apiKey]lastRequestTime
	delayPerReq time.Duration        // Thời gian đợi giữa các request (12 giây cho 5 req/min)
}

// NewInMemoryRateLimiter tạo rate limiter mới in-memory
func NewInMemoryRateLimiter() *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		lastRequest: make(map[string]time.Time),
		delayPerReq: 12 * time.Second, // 60 giây / 5 requests = 12 giây/request
	}
}

// WaitForKey đợi đến khi có thể thực hiện request với một API key cụ thể
// Đơn giản: đợi 12 giây kể từ request cuối cùng của key này
func (r *InMemoryRateLimiter) WaitForKey(apiKey string) error {
	keyHash := hashAPIKeyForRateLimit(apiKey)

	r.mu.Lock()
	lastTime, exists := r.lastRequest[keyHash]
	now := time.Now()

	if exists {
		// Tính thời gian đã trôi qua kể từ request cuối cùng
		elapsed := now.Sub(lastTime)
		if elapsed < r.delayPerReq {
			// Cần đợi thêm để đảm bảo 5 req/min (12 giây/request)
			waitTime := r.delayPerReq - elapsed
			r.mu.Unlock()
			log.Printf("Rate limit: đợi %.1f giây trước request tiếp theo (đã trôi qua %.1f giây, cần 12s giữa các request)",
				waitTime.Seconds(), elapsed.Seconds())
			time.Sleep(waitTime)
			r.mu.Lock()
			now = time.Now()
		} else {
			// Đã đợi đủ, không cần sleep
			log.Printf("Rate limit: OK (đã trôi qua %.1f giây, đủ để gọi request tiếp theo)", elapsed.Seconds())
		}
	} else {
		// Request đầu tiên, không cần đợi
		log.Printf("Rate limit: Request đầu tiên, không cần đợi")
	}

	// Cập nhật thời gian request cuối cùng
	r.lastRequest[keyHash] = now
	r.mu.Unlock()

	return nil
}

// Wait đợi đến khi có thể thực hiện request tiếp theo (wrapper)
func (r *InMemoryRateLimiter) Wait() error {
	return r.WaitForKey("default")
}

// Close đóng rate limiter (no-op cho in-memory)
func (r *InMemoryRateLimiter) Close() error {
	return nil
}

// hashAPIKeyForRateLimit tạo hash ngắn gọn từ API key để dùng làm key
func hashAPIKeyForRateLimit(apiKey string) string {
	// Sử dụng 8 ký tự đầu và cuối để tạo identifier
	if len(apiKey) <= 16 {
		return apiKey
	}
	return apiKey[:8] + apiKey[len(apiKey)-8:]
}

// Các hàm next_url không cần thiết nữa vì SDK tự handle
// Giữ lại để tương thích với interface nhưng không làm gì

// SaveNextURL không làm gì (SDK tự handle next_url)
func (r *InMemoryRateLimiter) SaveNextURL(ticker string, nextURL string) error {
	return nil
}

// GetNextURL luôn trả về empty (SDK tự handle next_url)
func (r *InMemoryRateLimiter) GetNextURL(ticker string) (string, error) {
	return "", nil
}

// DeleteNextURL không làm gì
func (r *InMemoryRateLimiter) DeleteNextURL(ticker string) error {
	return nil
}

// SaveTickerProgress không làm gì
func (r *InMemoryRateLimiter) SaveTickerProgress(ticker string, progress string) error {
	return nil
}

// GetTickerProgress luôn trả về empty
func (r *InMemoryRateLimiter) GetTickerProgress(ticker string) (string, error) {
	return "", nil
}
