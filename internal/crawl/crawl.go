package crawl

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"us-data/internal/provider"
	"us-data/internal/slogx"
)

// Job represents one crawl unit (ticker + date range)
type Job struct {
	Ticker string
	From   time.Time
	To     time.Time
}

// JobResult is sent by workers for fan-in
type JobResult struct {
	Ok        bool
	Ticker    string
	DateRange string
	Reason    string
	Bars      int
	KeyPrefix string
}

// Cmd triggers a crawl run
type Cmd struct{}

// Done signals crawl completion
type Done struct{}

// FilterTickersToCrawl returns jobs: no progress → full 2y; has progress → gap (lastdate+1..yesterday)
func FilterTickersToCrawl(tickers []string, progressPath string, now time.Time) []Job {
	m := loadProgress(progressPath)
	yesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)

	var jobs []Job
	for _, t := range tickers {
		last, ok := m[t]
		if !ok {
			from := time.Date(now.Year()-2, now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			to := now.AddDate(0, 0, 1)
			jobs = append(jobs, Job{Ticker: t, From: from, To: to})
			continue
		}
		start, _ := time.ParseInLocation("2006-01-02", last, time.UTC)
		start = start.AddDate(0, 0, 1)
		startDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		endDate := yesterday
		if startDate.After(endDate) {
			continue
		}
		for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
			dayEnd := d.Add(24*time.Hour - time.Millisecond)
			jobs = append(jobs, Job{Ticker: t, From: d, To: dayEnd})
		}
	}
	return jobs
}

// RunOneCrawl runs one crawl cycle in parallel mode, sends done when finished.
// Luôn dùng RunParallel (chan key + workers), kể cả khi chỉ có 1 key.
func RunOneCrawl(
	dp provider.DataProvider,
	apiKeys []string,
	tickers []string,
	saveBaseDir, saveFormat, progressPath string,
	progressUpdates chan<- ProgressUpdate,
	done chan<- Done,
	shutdown <-chan struct{},
) {
	now := time.Now().UTC()
	jobs := FilterTickersToCrawl(tickers, progressPath, now)
	if len(jobs) == 0 {
		slog.Info("no jobs to crawl, skip")
		done <- Done{}
		return
	}

	useSavePerDay := len(jobs) > 0 && jobs[0].To.Sub(jobs[0].From) < 25*time.Hour
	if p, ok := dp.(*provider.PolygonProvider); ok {
		p.SetSavePerDay(useSavePerDay)
	}

	seenTickers := make(map[string]bool)
	for _, j := range jobs {
		seenTickers[j.Ticker] = true
	}
	skipped := len(tickers) - len(seenTickers)
	if skipped > 0 {
		slog.Info("tickers up to date, jobs to crawl", "skipped", skipped, "jobs", len(jobs), "tickers", len(seenTickers))
	} else {
		slog.Info("jobs to crawl", "jobs", len(jobs))
	}

	var success, failed int
	var successList []string
	var failedList []failedEntry
	defer func() {
		if len(successList) > 0 || len(failedList) > 0 {
			if err := writeRunReport(saveBaseDir, successList, failedList); err != nil {
				slog.Warn("could not write run report", "error", err)
			} else {
				slog.Info("run report saved", "success", len(successList), "failed", len(failedList))
			}
		}
	}()

	success, failed, successList, failedList = RunParallel(dp, apiKeys, jobs, progressUpdates, shutdown)
	slog.Info("crawl done", "success", success, "failed", failed)
	done <- Done{}
}

func runJobResultCollector(
	results <-chan JobResult,
	mu *sync.Mutex,
	success, failed *int,
	barsPerTicker, barsPerKey map[string]int,
	successList *[]string,
	failedList *[]failedEntry,
) {
	for r := range results {
		mu.Lock()
		if r.Ok {
			*success++
			*successList = appendSuccess(*successList, r.Ticker)
			barsPerTicker[r.Ticker] += r.Bars
			barsPerKey[r.KeyPrefix] += r.Bars
		} else {
			*failed++
			*failedList = append(*failedList, failedEntry{Ticker: r.Ticker, DateRange: r.DateRange, Reason: r.Reason})
		}
		mu.Unlock()
	}
}

// RunParallel runs crawl with N workers and key pool
func RunParallel(
	dp provider.DataProvider,
	apiKeys []string,
	jobs []Job,
	progressUpdates chan<- ProgressUpdate,
	shutdown <-chan struct{},
) (successCount, failedCount int, successList []string, failedList []failedEntry) {
	p, ok := dp.(*provider.PolygonProvider)
	if !ok {
		slog.Error("RunParallel expects *PolygonProvider", "got", fmt.Sprintf("%T", dp))
		return 0, 0, nil, nil
	}
	crawler := p.Crawler
	if crawler == nil {
		slog.Error("RunParallel: PolygonProvider has nil crawler")
		return 0, 0, nil, nil
	}

	logs := make(chan string, 2048)
	logger := slogx.NewChanLogger(logs)
	errs := make(chan errorEntry, 64)
	var logWg sync.WaitGroup
	logWg.Add(1)
	go func() {
		defer logWg.Done()
		runLogWriter(logs)
	}()
	var errWg sync.WaitGroup
	errWg.Add(1)
	go func() {
		defer errWg.Done()
		runErrorHandler(errs, logger)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p.SetLogFunc(func(msg string) { logger.Info(msg) })
	defer func() {
		p.SetLogFunc(nil)
		close(logs)
		close(errs)
		logWg.Wait()
		errWg.Wait()
	}()

	pending := make(chan Job, len(jobs))
	for _, j := range jobs {
		pending <- j
	}
	close(pending)

	keyPool := make(chan string, len(apiKeys))
	for _, k := range apiKeys {
		keyPool <- k
	}

	results := make(chan JobResult, len(jobs)+64)
	var mu sync.Mutex
	var success, failed int
	barsPerTicker := make(map[string]int)
	barsPerKey := make(map[string]int)
	var successListPtr []string
	var failedListPtr []failedEntry
	var resWg sync.WaitGroup
	resWg.Add(1)
	go func() {
		defer resWg.Done()
		runJobResultCollector(results, &mu, &success, &failed, barsPerTicker, barsPerKey, &successListPtr, &failedListPtr)
	}()

	go runHeartbeat(ctx, 30*time.Second, len(jobs), &mu, &success, &failed, barsPerTicker, logger)

	var wg sync.WaitGroup
	wg.Add(len(apiKeys))
	for i := 0; i < len(apiKeys); i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-shutdown:
					return
				case job, ok := <-pending:
					if !ok {
						return
					}
					key := <-keyPool
					keyPrefix := key
					if len(key) > 8 {
						keyPrefix = key[:8]
					}
					logger.Info("key take", "ticker", job.Ticker, "key", keyPrefix)
					bars, err := crawler.CrawlMinuteBarsWithKey(job.Ticker, key, job.From, job.To)
					logger.Info("key return", "ticker", job.Ticker, "key", keyPrefix)
					keyPool <- key

					lastDateStr := job.To.Format("2006-01-02")
					dateRange := job.From.Format("2006-01-02") + ".." + job.To.Format("2006-01-02")
					if err != nil {
						reason := err.Error()
						logger.Error("crawl fail", "ticker", job.Ticker, "date_range", dateRange, "reason", reason)
						select {
						case errs <- errorEntry{Ticker: job.Ticker, Err: err}:
						default:
						}
						results <- JobResult{Ok: false, Ticker: job.Ticker, DateRange: dateRange, Reason: reason}
					} else if len(bars) == 0 {
						reason := "no data"
						logger.Error("crawl fail", "ticker", job.Ticker, "date_range", dateRange, "reason", reason)
						results <- JobResult{Ok: false, Ticker: job.Ticker, DateRange: dateRange, Reason: reason}
					} else {
						n := len(bars)
						logger.Info("crawl ok", "ticker", job.Ticker, "date_range", dateRange, "bars", n)
						results <- JobResult{Ok: true, Ticker: job.Ticker, DateRange: dateRange, Bars: n, KeyPrefix: keyPrefix}
						select {
						case progressUpdates <- ProgressUpdate{Ticker: job.Ticker, Date: lastDateStr}:
						default:
							logger.Warn("progress channel full, skip update", "ticker", job.Ticker)
						}
					}
				}
			}
		}()
	}
	wg.Wait()
	close(results)
	resWg.Wait()
	cancel()

	var total int
	for _, n := range barsPerTicker {
		total += n
	}
	logger.Info("summary", "total_bars", total, "success", success, "failed", failed)
	if len(barsPerTicker) > 0 {
		tickers := make([]string, 0, len(barsPerTicker))
		for t := range barsPerTicker {
			tickers = append(tickers, t)
		}
		sort.Strings(tickers)
		for _, t := range tickers {
			logger.Info("summary ticker", "ticker", t, "bars", barsPerTicker[t])
		}
	}
	if len(barsPerKey) > 0 {
		keys := make([]string, 0, len(barsPerKey))
		for k := range barsPerKey {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			logger.Info("summary key", "key", k, "bars", barsPerKey[k])
		}
	}
	if len(failedListPtr) > 0 {
		logger.Info("summary failed", "count", len(failedListPtr), "reasons", joinFailedReasons(failedListPtr))
	}

	return success, failed, successListPtr, failedListPtr
}
