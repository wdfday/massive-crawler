package saver

// Bar là DTO dùng cho lưu packet (CSV/Parquet/JSON).
// Package saver không phụ thuộc polygon hay provider — DIP.
type Bar struct {
	Timestamp    int64   `json:"t" parquet:"t"`
	Open         float64 `json:"o" parquet:"o"`
	High         float64 `json:"h" parquet:"h"`
	Low          float64 `json:"l" parquet:"l"`
	Close        float64 `json:"c" parquet:"c"`
	Volume       int64   `json:"v" parquet:"v"`
	VWAP         float64 `json:"vw,omitempty" parquet:"vw,optional"`
	Transactions int64   `json:"n,omitempty" parquet:"n,optional"`
}
