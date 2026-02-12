package polygon

import (
	"sync"
	"testing"
	"time"

	"us-data/internal/model"
)

const (
	benchBars2Years = 504 * 960 // ~484k bars
	benchNumKeys    = 3
)

// mockCrawl simulates crawl: sleep cooldown, return bars (pre-alloc)
func mockCrawl(cooldown time.Duration, withPrealloc bool) func(string, string) ([]model.Bar, error) {
	from := time.Now().AddDate(-2, 0, 0)
	to := time.Now()
	cap := estimatedBars(from, to)
	return func(ticker, key string) ([]model.Bar, error) {
		time.Sleep(cooldown)
		var bars []model.Bar
		if withPrealloc {
			bars = make([]model.Bar, 0, cap)
		}
		for j := 0; j < benchBars2Years; j++ {
			bars = append(bars, model.Bar{
				Timestamp: int64(j), Open: 100, High: 101, Low: 99, Close: 100.5,
				Volume: 1000, VWAP: 100.2, Transactions: 50,
			})
		}
		return bars, nil
	}
}

// runChanFlow 3 keys, indiceChan + keyChan, cooldown 12s (or 12ms for quick bench)
func runChanFlow(tickers []string, cooldown time.Duration, withPrealloc bool) {
	apiKeys := []string{"key1", "key2", "key3"}
	crawl := mockCrawl(cooldown, withPrealloc)

	indiceChan := make(chan string, len(tickers))
	for _, t := range tickers {
		indiceChan <- t
	}
	close(indiceChan)

	keyChan := make(chan string, len(apiKeys))
	for _, k := range apiKeys {
		keyChan <- k
	}

	var mu sync.Mutex
	var success, failed int
	var wg sync.WaitGroup
	wg.Add(len(apiKeys))

	for i := 0; i < len(apiKeys); i++ {
		go func() {
			defer wg.Done()
			for ticker := range indiceChan {
				key := <-keyChan
				bars, err := crawl(ticker, key)
				keyChan <- key
				mu.Lock()
				if err != nil || len(bars) == 0 {
					failed++
				} else {
					success++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
}

// BenchmarkChanFlowQuick 3 keys, 9 tickers, 12ms cooldown (simulating 12s) — runs fast
func BenchmarkChanFlowQuick(b *testing.B) {
	tickers := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"}
	cooldown := 12 * time.Millisecond
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runChanFlow(tickers, cooldown, true)
	}
}

// BenchmarkChanFlowReal 3 keys, 9 tickers, 12s cooldown — real case, long run (~36s/op)
// go test -bench=BenchmarkChanFlowReal -benchtime=1x -benchmem ./polygon/
func BenchmarkChanFlowReal(b *testing.B) {
	tickers := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"}
	cooldown := 12 * time.Second
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runChanFlow(tickers, cooldown, true)
	}
}

// BenchmarkChanFlowPreallocVsNoPrealloc compares pre-alloc in chan flow (12ms to run)
func BenchmarkChanFlowPreallocVsNoPrealloc(b *testing.B) {
	tickers := []string{"A", "B", "C"}
	cooldown := 12 * time.Millisecond

	b.Run("Prealloc", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			runChanFlow(tickers, cooldown, true)
		}
	})
	b.Run("NoPrealloc", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			runChanFlow(tickers, cooldown, false)
		}
	})
}

// BenchmarkAppendNoPrealloc simulates append bars without pre-alloc
func BenchmarkAppendNoPrealloc(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var allBars []model.Bar
		for j := 0; j < benchBars2Years; j++ {
			allBars = append(allBars, model.Bar{
				Timestamp: int64(j),
				Open:      100, High: 101, Low: 99, Close: 100.5,
				Volume: 1000, VWAP: 100.2, Transactions: 50,
			})
		}
		_ = allBars
	}
}

// BenchmarkAppendPrealloc simulates append bars with pre-alloc
func BenchmarkAppendPrealloc(b *testing.B) {
	from := time.Now().AddDate(-2, 0, 0)
	to := time.Now()
	cap := estimatedBars(from, to)
	for i := 0; i < b.N; i++ {
		allBars := make([]model.Bar, 0, cap)
		for j := 0; j < benchBars2Years; j++ {
			allBars = append(allBars, model.Bar{
				Timestamp: int64(j),
				Open:      100, High: 101, Low: 99, Close: 100.5,
				Volume: 1000, VWAP: 100.2, Transactions: 50,
			})
		}
		_ = allBars
	}
}
