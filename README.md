# US Data Crawler

Go application that crawls minute OHLCV bars for US stocks from Polygon API. Supports full 2-year historical crawl and incremental daily updates.

## Features

- **Polygon API** – minute aggregates, rate limit 5 req/min per key
- **Chan model** – indiceChan + keyChan, workers = API keys, 12s cooldown per key
- **Unified crawl** – no progress → full 2y; has progress → fill gap (lastdate+1 .. yesterday)
- **Progress file** – `.lastday.json` per ticker (fan-in from workers)
- **Output formats** – Parquet, CSV, JSON
- **Graceful shutdown** – SIGINT/SIGTERM, finish current jobs then exit
- **Fan-in patterns** – progress, log, error; heartbeat; summary (bars per ticker, per key)

## Quick Start

```bash
cp .env.example .env
# Edit .env: add POLYGON_API_KEYS (or POLYGON_API_KEY)

# Fetch tickers (S&P 500 + NASDAQ 100)
bash scripts/fetch_indices.sh

# Run
go run ./cmd/us-data/
```

## Docker

```bash
cp .env.example .env
# Edit .env: add POLYGON_API_KEYS (or POLYGON_API_KEY)

bash scripts/fetch_indices.sh

docker compose up --build -d
docker compose logs -f crawler
```

Data is stored in `./data` (or `DATA_DIR_HOST`). Progress persists across restarts.

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `POLYGON_API_KEY` | Single API key (alternative to `POLYGON_API_KEYS`) | - |
| `POLYGON_API_KEYS` | One or more API keys (comma-separated); number of workers = number of keys | (required\*) |
| `SAVE_FORMAT` | `parquet` \| `csv` \| `json` | `parquet` |
| `DATA_DIR` | Data directory (local) | `data` |
| `DATA_DIR_HOST` | Host path for Docker volume | `./data` |
| `STOCK_SELECTION` | Currently only `file` is used; tickers are loaded from a file/indices list | `file` |
| `TICKERS_FILE` | Path to ticker list (when `file`); falls back to `indices/combined.txt` when empty | `indices/combined.txt` |
| `PHASE2_RUN_HOUR` | Next run hour (UTC 0-23) | `0` |
| `PHASE2_RUN_MINUTE` | Next run minute (0-59) | `30` |

## Output

```
data/Polygon/{Ticker}/{ticker}_{from}_to_{to}.{ext}
data/Polygon/{Ticker}/{ticker}_{date}.{ext}   # per-day (gap fill)
data/Polygon/.lastday.json                    # progress
```

Example: `data/Polygon/AAPL/AAPL_2024-02-05_to_2026-02-05.parquet`

## Project Structure (Option A)

```
├── cmd/us-data/      # Entry point
├── internal/
│   ├── app/          # config, phase, setup (bootstrap)
│   ├── crawl/        # crawl, fanin, progress, report
│   ├── provider/     # DataProvider + polygon (Crawler, transport, indices, types)
│   └── saver/        # Packet savers (Parquet, CSV, JSON)
├── indices/          # Ticker lists (combined.txt, sp500.txt)
├── scripts/          # fetch_indices.sh
└── docs/             # DEBUG.md, DIAGRAMS.md
```

## Debug

See [docs/DEBUG.md](docs/DEBUG.md) for GODEBUG, GC trace, alloc trace, Docker commands.

```bash
# GC trace
GODEBUG=gctrace=1 go run ./cmd/us-data/

# With Docker
GODEBUG=gctrace=1 docker compose up
```

## License

MIT
