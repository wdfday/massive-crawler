//go:build wireinject
// +build wireinject

package main

import (
	"us-data/internal/app"
	"us-data/internal/provider"

	"github.com/google/wire"
)

// App holds application dependencies built by Wire.
type App struct {
	Config *app.Config
	DP     provider.DataProvider
}

// InitializeApp builds App (Config + DataProvider) via Wire.
// Caller must call a.DP.Close() when done.
func InitializeApp() (*App, error) {
	wire.Build(
		app.ProvideConfig,
		app.ProvidePacketSaver,
		app.ProvidePolygonProvider,
		wire.Bind(new(provider.DataProvider), new(*provider.PolygonProvider)),
		wire.Struct(new(App), "Config", "DP"),
	)
	return nil, nil
}
