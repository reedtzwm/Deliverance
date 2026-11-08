package main

import (
	"golang.org/x/sys/windows/registry"

	"github.com/wailsapp/wails/v2/pkg/options"
)

// SetTheme сохраняет выбор темы: auto | light | dark.
func (a *App) SetTheme(theme string) {
	if theme != "light" && theme != "dark" {
		theme = "auto"
	}
	c := loadConfig()
	c.Theme = theme
	_ = saveConfig(c)
}

// systemDark: включён ли тёмный режим приложений в Windows.
func systemDark() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && v == 0
}

// windowBackground — цвет окна до загрузки страницы (чтобы ночью не было белой вспышки).
func windowBackground(theme string) *options.RGBA {
	dark := theme == "dark" || (theme != "light" && systemDark())
	if dark {
		return &options.RGBA{R: 14, G: 14, B: 14, A: 255}
	}
	return &options.RGBA{R: 255, G: 255, B: 255, A: 255}
}
