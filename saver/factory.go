package saver

import (
	"fmt"
	"strings"
)

// NewPacketSaver tạo implementation theo format (csv, parquet, json).
// Trả về nil nếu format không hỗ trợ.
func NewPacketSaver(format string) PacketSaver {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "csv":
		return CSVSaver{}
	case "parquet":
		return ParquetSaver{}
	case "json":
		return JSONSaver{}
	default:
		return nil
	}
}

// MustPacketSaver giống NewPacketSaver nhưng panic nếu format không hợp lệ.
func MustPacketSaver(format string) PacketSaver {
	s := NewPacketSaver(format)
	if s == nil {
		panic(fmt.Sprintf("saver: format không hỗ trợ %q (dùng: csv, parquet, json)", format))
	}
	return s
}
