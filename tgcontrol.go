package main

import (
	"crypto/rand"
	"encoding/hex"
	"os/exec"
)

func tgLink(secret string) string {
	if secret == "" {
		return ""
	}
	return "tg://proxy?server=127.0.0.1&port=" + itoa(tgDefaultPort) + "&secret=dd" + secret
}

// startTg запускает прокси с сохранённым секретом (создаёт его при первом включении).
func (a *App) startTg() {
	c := loadConfig()
	if c.TgSecret == "" {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		c.TgSecret = hex.EncodeToString(b)
		_ = saveConfig(c)
	}
	if err := a.tg.Start(c.TgSecret, tgDefaultPort); err != nil {
		a.logf("Прокси для Telegram не запущен: " + err.Error() +
			" (порт " + itoa(tgDefaultPort) + " может быть занят, например программой TG WS Proxy)")
		return
	}
	a.logf("Прокси для Telegram запущен на 127.0.0.1:" + itoa(tgDefaultPort))
}

// SetTgProxy включает или выключает прокси и запоминает выбор.
func (a *App) SetTgProxy(enable bool) {
	c := loadConfig()
	c.TgProxy = enable
	_ = saveConfig(c)
	if enable {
		a.startTg()
	} else {
		a.tg.Stop()
		a.logf("Прокси для Telegram остановлен")
	}
	a.emitState()
}

// OpenTgLink открывает ссылку tg://proxy в Telegram, чтобы он добавил прокси.
func (a *App) OpenTgLink() {
	c := loadConfig()
	if !a.tg.Running() {
		a.logf("Сначала включите прокси")
		return
	}
	_ = hidden(exec.Command("rundll32", "url.dll,FileProtocolHandler", tgLink(c.TgSecret))).Start()
}
