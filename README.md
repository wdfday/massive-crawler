# US Data Crawler

Crawler minute bars (OHLCV) cho US stocks từ now − 2 năm đến hiện tại. Polygon API, 1 file/ticker (CSV/Parquet/JSON).

## Chạy nhanh

```bash
cp .env.example .env
# Sửa .env, thêm POLYGON_API_KEY

# Lấy danh sách ticker (S&P 500 + NASDAQ 100)
bash scripts/fetch_indices.sh

go run .
```

## Docker

```bash
cp .env.example .env
docker compose up --build -d
docker compose logs -f crawler
```

## Cấu hình chính

| Biến | Mô tả | Mặc định |
|------|-------|----------|
| `POLYGON_API_KEY` | API key | (bắt buộc) |
| `SAVE_FORMAT` | `csv` \| `parquet` \| `json` | `parquet` |
| `DATA_DIR` | Thư mục lưu | `data` |
| `STOCK_SELECTION` | `file` \| `top-marketcap` \| `top-volume` \| `sp500` \| `any` | `file` |

## Output

`data/Polygon/{Ticker}/{ticker}_{from}_to_{to}.{ext}`

Ví dụ: `data/Polygon/AAPL/AAPL_2024-01-30_to_2026-01-30.parquet`
