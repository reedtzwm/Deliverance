package main

import (
	"os"
	"path/filepath"
)

// dataDir — единая папка данных (%ProgramData%\Deliverance).
// Сам exe лежит в Program Files, все скачанные файлы zapret и настройки — здесь.
// Папка прежней версии (ZapretGUI) переносится сюда при первом запуске.
func dataDir() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\ProgramData`
	}
	d := filepath.Join(base, "Deliverance")
	old := filepath.Join(base, "ZapretGUI")
	if _, err := os.Stat(d); err != nil {
		if _, oerr := os.Stat(old); oerr == nil {
			if os.Rename(old, d) != nil {
				// файлы заняты (например, работает winws) — продолжаем со старой папкой
				return old
			}
		}
	}
	_ = os.MkdirAll(d, 0o755)
	return d
}

func zapretDir() string   { return filepath.Join(dataDir(), "zapret") }
func configPath() string  { return filepath.Join(dataDir(), "config.json") }
func versionPath() string { return filepath.Join(zapretDir(), ".version") }
