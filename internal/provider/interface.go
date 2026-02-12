package provider

// DataProvider is the abstraction used by the application when accessing a data source.
// Implementations are responsible for their own internal crawl logic and resource cleanup.
type DataProvider interface {
	GetName() string
	Close() error
}
