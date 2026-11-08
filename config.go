package main

import (
	"encoding/json"
	"os"
	"sync"
)

type Config struct {
	SelectedBat   string `json:"selectedBat"`   // имя выбранного .bat
	AutoUpdate    bool   `json:"autoUpdate"`    // проверять обновления при старте
	AutoStart     bool   `json:"autoStart"`     // запускать zapret при старте приложения
	AutoSelect    bool   `json:"autoSelect"`    // подбирать батник, если выбранный не работает
	AutoRun       bool   `json:"autoRun"`       // запускать программу вместе с Windows
	Watchdog      bool   `json:"watchdog"`      // периодически проверять работоспособность
	CustomDomains string `json:"customDomains"` // свои домены, по одному в строке
	GameFilter    string `json:"gameFilter"`    // off | all | tcp | udp
	Theme         string `json:"theme"`         // auto | light | dark
	TgProxy       bool   `json:"tgProxy"`       // локальный прокси для Telegram включён
	TgSecret      string `json:"tgSecret"`      // секрет прокси (32 hex), генерируется один раз
}

var cfgMu sync.Mutex

func defaultConfig() Config {
	return Config{AutoUpdate: true, AutoStart: true, AutoSelect: true, Theme: "auto"}
}

func loadConfig() Config {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	c := defaultConfig()
	if b, err := os.ReadFile(configPath()); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c
}

func saveConfig(c Config) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), b, 0o644)
}
