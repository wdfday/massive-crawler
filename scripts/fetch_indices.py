#!/usr/bin/env python3
"""
Script để lấy danh sách S&P 500 và NASDAQ 100 từ Wikipedia
Sử dụng Python với requests và BeautifulSoup để parse HTML
"""

import json
import re
import requests
from bs4 import BeautifulSoup
from pathlib import Path

OUTPUT_DIR = Path("indices")
OUTPUT_DIR.mkdir(exist_ok=True)

def fetch_sp500():
    """Lấy danh sách S&P 500 từ Wikipedia"""
    print("Đang lấy danh sách S&P 500 từ Wikipedia...")
    
    url = "https://en.wikipedia.org/wiki/List_of_S%26P_500_companies"
    response = requests.get(url, headers={'User-Agent': 'Mozilla/5.0'})
    response.raise_for_status()
    
    soup = BeautifulSoup(response.content, 'html.parser')
    
    # Tìm table chứa tickers
    table = soup.find('table', {'id': 'constituents'})
    if not table:
        # Thử cách khác
        table = soup.find('table', class_='wikitable')
    
    tickers = []
    if table:
        # Lấy cột đầu tiên (ticker symbol)
        rows = table.find_all('tr')[1:]  # Bỏ header
        for row in rows:
            cells = row.find_all('td')
            if cells:
                ticker = cells[0].get_text(strip=True)
                if ticker and ticker.isupper():
                    tickers.append(ticker)
    
    # Fallback: tìm tất cả data-symbol attributes
    if not tickers:
        for tag in soup.find_all(attrs={'data-symbol': True}):
            ticker = tag.get('data-symbol')
            if ticker and ticker.isupper():
                tickers.append(ticker)
    
    # Fallback: regex pattern
    if not tickers:
        pattern = r'data-symbol="([A-Z]+)"'
        matches = re.findall(pattern, response.text)
        tickers = list(set(matches))
    
    tickers = sorted(set(tickers))
    
    # Lưu vào file
    sp500_file = OUTPUT_DIR / "sp500.txt"
    with open(sp500_file, 'w') as f:
        f.write('\n'.join(tickers))
    
    print(f"Đã lấy được {len(tickers)} tickers từ S&P 500")
    print(f"  (Lưu ý: S&P 500 có ~503 tickers vì một số công ty có nhiều class shares)")
    print(f"  (Ví dụ: GOOG/GOOGL, FOX/FOXA, NWS/NWSA)")
    return tickers

def fetch_nasdaq100():
    """Lấy danh sách NASDAQ 100 từ Wikipedia"""
    print("Đang lấy danh sách NASDAQ 100 từ Wikipedia...")
    
    url = "https://en.wikipedia.org/wiki/NASDAQ-100"
    response = requests.get(url, headers={'User-Agent': 'Mozilla/5.0'})
    response.raise_for_status()
    
    soup = BeautifulSoup(response.content, 'html.parser')
    
    tickers = []
    
    # Tìm table chứa tickers
    tables = soup.find_all('table', class_='wikitable')
    for table in tables:
        rows = table.find_all('tr')[1:]  # Bỏ header
        for row in rows:
            cells = row.find_all('td')
            if cells:
                # Thường ticker ở cột đầu tiên hoặc cột thứ hai
                for cell in cells[:2]:
                    text = cell.get_text(strip=True)
                    # Kiểm tra xem có phải ticker không (thường là 1-5 ký tự, uppercase)
                    if text and len(text) <= 5 and text.isupper() and text.isalpha():
                        tickers.append(text)
                        break
    
    # Fallback: regex pattern
    if not tickers:
        pattern = r'data-symbol="([A-Z]+)"'
        matches = re.findall(pattern, response.text)
        tickers = list(set(matches))
    
    tickers = sorted(set(tickers))
    
    # Lưu vào file
    nasdaq100_file = OUTPUT_DIR / "nasdaq100.txt"
    with open(nasdaq100_file, 'w') as f:
        f.write('\n'.join(tickers))
    
    print(f"Đã lấy được {len(tickers)} tickers từ NASDAQ 100")
    print(f"  (Lưu ý: NASDAQ 100 có thể có nhiều hơn 100 tickers do:")
    print(f"   - Một số công ty có nhiều class shares")
    print(f"   - Wikipedia có thể list cả các tickers phụ)")
    return tickers

def main():
    try:
        sp500 = fetch_sp500()
        nasdaq100 = fetch_nasdaq100()
        
        # Gộp và loại bỏ duplicate
        combined = sorted(set(sp500 + nasdaq100))
        
        # Lưu combined
        combined_file = OUTPUT_DIR / "combined.txt"
        with open(combined_file, 'w') as f:
            f.write('\n'.join(combined))
        
        # Lưu JSON
        json_file = OUTPUT_DIR / "tickers.json"
        with open(json_file, 'w') as f:
            json.dump(combined, f, indent=2)
        
        print(f"\nHoàn thành!")
        print(f"- S&P 500: {len(sp500)} tickers")
        print(f"- NASDAQ 100: {len(nasdaq100)} tickers")
        print(f"- Combined (unique): {len(combined)} tickers")
        print(f"\nFiles đã được lưu trong thư mục {OUTPUT_DIR}/")
        print(f"- sp500.txt: S&P 500 tickers")
        print(f"- nasdaq100.txt: NASDAQ 100 tickers")
        print(f"- combined.txt: Gộp cả hai (unique)")
        print(f"- tickers.json: JSON format")
        
    except Exception as e:
        print(f"Lỗi: {e}")
        return 1
    
    return 0

if __name__ == "__main__":
    exit(main())
