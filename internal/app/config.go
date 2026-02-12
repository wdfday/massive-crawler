package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds application configuration from env
type Config struct {
	DataProvider    string
	StockSelection  string
	TickersFile     string
	DataDir         string
	SaveFormat      string
	LogLevel        string // debug | info | warn | error
	PolygonAPIKeys  []string
	Phase2RunHour   int
	Phase2RunMinute int
}

// Load reads config from environment
func LoadConfig() *Config {
	cfg := &Config{
		DataProvider:    getEnv("DATA_PROVIDER", "polygon"),
		StockSelection:  getEnv("STOCK_SELECTION", "file"),
		TickersFile:     os.Getenv("TICKERS_FILE"),
		DataDir:         getEnv("DATA_DIR", "data"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		Phase2RunHour:   0,
		Phase2RunMinute: 30,
	}
	cfg.SaveFormat = getSaveFormat()
	cfg.PolygonAPIKeys = parsePolygonAPIKeys()
	if h := os.Getenv("PHASE2_RUN_HOUR"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v >= 0 && v <= 23 {
			cfg.Phase2RunHour = v
		}
	}
	if m := os.Getenv("PHASE2_RUN_MINUTE"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v >= 0 && v <= 59 {
			cfg.Phase2RunMinute = v
		}
	}
	return cfg
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getSaveFormat() string {
	if v := os.Getenv("SAVE_FORMAT"); v != "" {
		return v
	}
	switch os.Getenv("PROFILE") {
	case "dev", "development":
		return "csv"
	case "prod", "production", "":
		return "parquet"
	default:
		return "parquet"
	}
}

func parsePolygonAPIKeys() []string {
	s := os.Getenv("POLYGON_API_KEYS")
	if s == "" {
		s = os.Getenv("POLYGON_API_KEY")
	}
	if s == "" {
		return nil
	}
	keys := strings.Split(s, ",")
	for i := range keys {
		keys[i] = strings.TrimSpace(keys[i])
	}
	return keys
}

// SaveBaseDir returns data/Polygon
func (c *Config) SaveBaseDir() string {
	return filepath.Join(c.DataDir, "Polygon")
}

// ProgressPath returns path to .lastday.json
func (c *Config) ProgressPath() string {
	return filepath.Join(c.SaveBaseDir(), ".lastday.json")
}
