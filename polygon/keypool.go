package polygon

import (
	"fmt"
	"sync"
	"time"
)

// APIKeyInfo chứa thông tin về một API key
type APIKeyInfo struct {
	Key          string
	LastUsed     time.Time
	RequestCount int64
}

// APIKeyPool quản lý pool của nhiều API keys
type APIKeyPool struct {
	keys        []*APIKeyInfo
	mu          sync.RWMutex
	index       int // Cho round-robin
	strategy    KeySelectionStrategy
	rateLimiter RateLimiter
}

// KeySelectionStrategy định nghĩa cách chọn API key
type KeySelectionStrategy int

const (
	RoundRobin KeySelectionStrategy = iota // Round-robin: luân phiên giữa các keys
	LeastUsed                              // Least-used: chọn key ít được dùng nhất
)

// NewAPIKeyPool tạo pool mới với danh sách API keys
func NewAPIKeyPool(apiKeys []string, strategy KeySelectionStrategy) (*APIKeyPool, error) {
	if len(apiKeys) == 0 {
		return nil, fmt.Errorf("cần ít nhất một API key")
	}

	// Tạo in-memory rate limiter
	rateLimiter := NewInMemoryRateLimiter()

	// Tạo key info cho mỗi API key
	keys := make([]*APIKeyInfo, len(apiKeys))
	for i, key := range apiKeys {
		keys[i] = &APIKeyInfo{
			Key:          key,
			LastUsed:     time.Time{},
			RequestCount: 0,
		}
	}

	return &APIKeyPool{
		keys:        keys,
		index:       0,
		strategy:    strategy,
		rateLimiter: rateLimiter,
	}, nil
}

// GetAvailableKey chọn một API key có sẵn dựa trên strategy
func (p *APIKeyPool) GetAvailableKey() (*APIKeyInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var selectedKey *APIKeyInfo

	switch p.strategy {
	case RoundRobin:
		// Round-robin: chọn key tiếp theo
		selectedKey = p.keys[p.index]
		p.index = (p.index + 1) % len(p.keys)

	case LeastUsed:
		// Least-used: chọn key có request count thấp nhất
		selectedKey = p.keys[0]
		for _, key := range p.keys[1:] {
			if key.RequestCount < selectedKey.RequestCount {
				selectedKey = key
			}
		}
	}

	// Kiểm tra rate limit cho key này
	if err := p.rateLimiter.WaitForKey(selectedKey.Key); err != nil {
		return nil, fmt.Errorf("key %s đã đạt rate limit: %w", selectedKey.Key[:8]+"...", err)
	}

	// Cập nhật thông tin key
	selectedKey.LastUsed = time.Now()
	selectedKey.RequestCount++

	return selectedKey, nil
}

// Wait đợi đến khi có key available (wrapper cho compatibility)
func (p *APIKeyPool) Wait() error {
	_, err := p.GetAvailableKey()
	return err
}

// GetRateLimiter trả về rate limiter
func (p *APIKeyPool) GetRateLimiter() RateLimiter {
	return p.rateLimiter
}

// Close đóng các kết nối
func (p *APIKeyPool) Close() error {
	if p.rateLimiter != nil {
		return p.rateLimiter.Close()
	}
	return nil
}

// GetStats trả về thống kê sử dụng của các keys
func (p *APIKeyPool) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_keys"] = len(p.keys)
	stats["strategy"] = p.strategy.String()

	keyStats := make([]map[string]interface{}, len(p.keys))
	for i, key := range p.keys {
		keyStats[i] = map[string]interface{}{
			"key_prefix":    key.Key[:8] + "...",
			"request_count": key.RequestCount,
			"last_used":     key.LastUsed.Format(time.RFC3339),
		}
	}
	stats["keys"] = keyStats

	return stats
}

// String implement Stringer cho KeySelectionStrategy
func (k KeySelectionStrategy) String() string {
	switch k {
	case RoundRobin:
		return "round-robin"
	case LeastUsed:
		return "least-used"
	default:
		return "unknown"
	}
}
