package saver

// PacketSaver là abstraction cho lưu từng packet (chunk) bars.
// High-level (main) inject implementation; low-level (crawler) chỉ phụ thuộc interface — DIP.
type PacketSaver interface {
	Save(bars []Bar, path string) error
	Extension() string
}
