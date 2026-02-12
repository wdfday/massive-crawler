package slogx

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
)

// ChanWriter buffers writes and sends complete lines to channel.
// Used with slog.TextHandler for fan-in logging.
type ChanWriter struct {
	Ch  chan<- string
	Buf []byte
}

func (w *ChanWriter) Write(p []byte) (n int, err error) {
	w.Buf = append(w.Buf, p...)
	for {
		i := bytes.IndexByte(w.Buf, '\n')
		if i < 0 {
			break
		}
		line := string(w.Buf[:i])
		w.Buf = w.Buf[i+1:]
		select {
		case w.Ch <- line:
		default:
			// channel full, drop
		}
	}
	return len(p), nil
}

// NewChanLogger creates a slog.Logger that writes to the channel in text format.
func NewChanLogger(ch chan<- string) *slog.Logger {
	w := &ChanWriter{Ch: ch}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// Default logger for direct use (writes to stderr, level info).
var Default = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// ParseLevel converts string (debug|info|warn|error) to slog.Level. Unknown → info.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewDefault creates a logger writing to stderr with the given level string.
func NewDefault(level string) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: ParseLevel(level),
	}))
}
