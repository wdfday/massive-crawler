#!/bin/bash

# Script để lấy danh sách S&P 500 và NASDAQ 100 từ Wikipedia
# Tương thích với macOS (BSD grep) và Linux (GNU grep)

OUTPUT_DIR="indices"
mkdir -p "$OUTPUT_DIR"

echo "Đang lấy danh sách S&P 500 từ Wikipedia..."

# Lấy S&P 500 từ Wikipedia - sử dụng sed thay vì grep -P để tương thích macOS
curl -s "https://en.wikipedia.org/wiki/List_of_S%26P_500_companies" | \
  grep -o 'data-symbol="[^"]*"' | \
  sed 's/data-symbol="//g' | sed 's/"//g' | \
  sort -u > "$OUTPUT_DIR/sp500.txt"

# Nếu không lấy được bằng cách trên, thử cách khác
if [ ! -s "$OUTPUT_DIR/sp500.txt" ]; then
  echo "Thử cách khác để lấy S&P 500..."
  curl -s "https://en.wikipedia.org/wiki/List_of_S%26P_500_companies" | \
    grep -E '^[[:space:]]*<td[^>]*>([A-Z]{1,5})</td>' | \
    sed -E 's/.*>([A-Z]{1,5})<.*/\1/' | \
    grep -E '^[A-Z]{1,5}$' | \
    sort -u > "$OUTPUT_DIR/sp500.txt"
fi

# Nếu vẫn không được, thử parse table trực tiếp
if [ ! -s "$OUTPUT_DIR/sp500.txt" ]; then
  echo "Thử parse table trực tiếp..."
  curl -s "https://en.wikipedia.org/wiki/List_of_S%26P_500_companies" | \
    awk '/<table.*id="constituents"/,/<\/table>/' | \
    grep -E '<td[^>]*>([A-Z]{1,5})</td>' | \
    sed -E 's/.*>([A-Z]{1,5})<.*/\1/' | \
    grep -E '^[A-Z]{1,5}$' | \
    head -503 | \
    sort -u > "$OUTPUT_DIR/sp500.txt"
fi

SP500_COUNT=$(wc -l < "$OUTPUT_DIR/sp500.txt" | tr -d ' ')
echo "Đã lấy được $SP500_COUNT tickers từ S&P 500"

echo "Đang lấy danh sách NASDAQ 100 từ Wikipedia..."

# Lấy NASDAQ 100 từ Wikipedia
curl -s "https://en.wikipedia.org/wiki/NASDAQ-100" | \
  grep -o 'data-symbol="[^"]*"' | \
  sed 's/data-symbol="//g' | sed 's/"//g' | \
  sort -u > "$OUTPUT_DIR/nasdaq100.txt"

# Nếu không lấy được, thử cách khác
if [ ! -s "$OUTPUT_DIR/nasdaq100.txt" ]; then
  echo "Thử cách khác để lấy NASDAQ 100..."
  curl -s "https://en.wikipedia.org/wiki/NASDAQ-100" | \
    grep -E '<td[^>]*>([A-Z]{1,5})</td>' | \
    sed -E 's/.*>([A-Z]{1,5})<.*/\1/' | \
    grep -E '^[A-Z]{1,5}$' | \
    sort -u > "$OUTPUT_DIR/nasdaq100.txt"
fi

NASDAQ_COUNT=$(wc -l < "$OUTPUT_DIR/nasdaq100.txt" | tr -d ' ')
echo "Đã lấy được $NASDAQ_COUNT tickers từ NASDAQ 100"

# Nếu cả hai đều rỗng, khuyên dùng Python script
if [ ! -s "$OUTPUT_DIR/sp500.txt" ] && [ ! -s "$OUTPUT_DIR/nasdaq100.txt" ]; then
  echo ""
  echo "⚠️  Cảnh báo: Không thể lấy dữ liệu bằng bash script trên macOS."
  echo "Vui lòng sử dụng Python script thay thế:"
  echo "  python3 scripts/fetch_indices.py"
  echo ""
  echo "Hoặc cài đặt GNU grep:"
  echo "  brew install grep"
  echo "  export PATH=\"/usr/local/opt/grep/libexec/gnubin:\$PATH\""
  exit 1
fi

# Gộp và loại bỏ duplicate
echo "Đang gộp danh sách..."
cat "$OUTPUT_DIR/sp500.txt" "$OUTPUT_DIR/nasdaq100.txt" 2>/dev/null | sort -u > "$OUTPUT_DIR/combined.txt"

COMBINED_COUNT=$(wc -l < "$OUTPUT_DIR/combined.txt" | tr -d ' ')
echo "Tổng cộng có $COMBINED_COUNT unique tickers"

# Tạo file JSON (cần jq, nếu không có thì bỏ qua)
if command -v jq &> /dev/null; then
  echo "Đang tạo file JSON..."
  cat "$OUTPUT_DIR/combined.txt" | jq -R -s 'split("\n") | map(select(length > 0))' > "$OUTPUT_DIR/tickers.json"
else
  echo "⚠️  jq không được cài đặt, bỏ qua tạo file JSON"
  echo "Có thể cài đặt: brew install jq"
fi

echo ""
echo "Hoàn thành! Files đã được lưu trong thư mục $OUTPUT_DIR/"
echo "- sp500.txt: S&P 500 tickers ($SP500_COUNT tickers)"
echo "- nasdaq100.txt: NASDAQ 100 tickers ($NASDAQ_COUNT tickers)"
echo "- combined.txt: Gộp cả hai (unique) ($COMBINED_COUNT tickers)"
if [ -f "$OUTPUT_DIR/tickers.json" ]; then
  echo "- tickers.json: JSON format"
fi
