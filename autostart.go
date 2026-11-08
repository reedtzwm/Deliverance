package main

import (
	"os"
	"os/exec"
)

const (
	taskName       = "Deliverance"
	legacyTaskName = "ZapretGUI" // прежнее имя до переименования
)

// Автозапуск через планировщик с наивысшими правами: записи в HKCU\Run
// для программ с запросом UAC Windows при входе пропускает.
// migrateLegacy: после переименования убирает старую задачу автозапуска и,
// если автозапуск был включён, создаёт новую уже с новым exe.
func migrateLegacy() {
	_ = hidden(exec.Command("schtasks", "/Delete", "/TN", legacyTaskName, "/F")).Run()
	if loadConfig().AutoRun {
		setAutostart(true)
	}
}

func setAutostart(enable bool) {
	if !enable {
		_ = hidden(exec.Command("schtasks", "/Delete", "/TN", taskName, "/F")).Run()
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = hidden(exec.Command("schtasks", "/Create", "/TN", taskName,
		"/TR", `"`+exe+`" --minimized`, "/SC", "ONLOGON", "/RL", "HIGHEST", "/F")).Run()
}
