package main

import (
	"embed"
	"encoding/json"
	"net/http"

	"github.com/Gonie-Gonie/idf-analyzer/internal/idf"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "IDF Analyzer",
		Width:  1600,
		Height: 900,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: appAssetHandler(),
		},
		BackgroundColour: &options.RGBA{R: 247, G: 249, B: 251, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}

func appAssetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/summary-metric-guides":
			if err := json.NewEncoder(w).Encode(idf.SummaryGuides()); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/api/settings":
			path, settings, err := loadAppSettings()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := json.NewEncoder(w).Encode(SettingsResult{Path: path, Settings: settings}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		default:
			http.NotFound(w, r)
		}
	})
}
