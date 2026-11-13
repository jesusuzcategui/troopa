package main

import (
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/jesusuzcategui/troopa/desktop"
	"github.com/jesusuzcategui/troopa/internal/app"
	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func main() {
	_, configPath, dbPath, err := config.Paths()
	if err != nil {
		log.Fatalf("troopa-desktop: paths: %v", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("troopa-desktop: config: %v", err)
	}
	s, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("troopa-desktop: store: %v", err)
	}
	defer func() { _ = s.Close() }()

	i18n.Apply(cfg.Language)
	svc := app.New(s, cfg, configPath)
	desktopSvc := NewDesktopService(nil, cfg, configPath)

	wailsApp := application.New(application.Options{
		Name:        "Troopa",
		Description: i18n.T("Your AI agent troop."),
		Services: []application.Service{
			application.NewService(svc),
			application.NewService(desktopSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(desktop.Assets()),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	desktopSvc.wailsApp = wailsApp

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Troopa",
		Width:            1280,
		Height:           800,
		MinWidth:         900,
		MinHeight:        560,
		BackgroundColour: application.NewRGB(30, 30, 46),
		URL:              "/",
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
