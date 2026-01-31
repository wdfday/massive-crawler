# Scripts để lấy danh sách S&P 500 và NASDAQ 100

## Cách sử dụng

### Option 1: Bash script (không cần dependencies)

```bash
bash scripts/fetch_indices.sh
```

Script sẽ:
- Lấy S&P 500 từ Wikipedia
- Lấy NASDAQ 100 từ Wikipedia  
- Gộp và loại bỏ duplicate
- Lưu vào thư mục `indices/`:
  - `sp500.txt` - S&P 500 tickers
  - `nasdaq100.txt` - NASDAQ 100 tickers
  - `combined.txt` - Gộp cả hai (unique)
  - `tickers.json` - JSON format

### Option 2: Python script (cần requests và beautifulsoup4)

```bash
# Cài đặt dependencies
pip install requests beautifulsoup4

# Chạy script
python3 scripts/fetch_indices.py
```

## Output

Sau khi chạy script, bạn sẽ có các file trong thư mục `indices/`:

- `sp500.txt`: Một ticker mỗi dòng
- `nasdaq100.txt`: Một ticker mỗi dòng
- `combined.txt`: Gộp cả hai, đã loại bỏ duplicate
- `tickers.json`: Array JSON format

## Sử dụng với crawler

Sau khi có file indices, crawler sẽ tự động đọc từ `indices/combined.txt` hoặc `indices/tickers.json`:

```bash
# Mặc định đọc từ file
go run main.go

# Hoặc chỉ định file cụ thể
export TICKERS_FILE="indices/combined.txt"
go run main.go
```

## Lưu ý

- Scripts lấy dữ liệu từ Wikipedia, có thể cần cập nhật định kỳ
- Nếu Wikipedia thay đổi cấu trúc HTML, có thể cần cập nhật script
- File `combined.txt` thường có khoảng 500-600 tickers (do một số tickers xuất hiện trong cả hai indices)
