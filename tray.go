package main

import (
	_ "embed"

	"fyne.io/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

var (
	mToggle *systray.MenuItem
	mTg     *systray.MenuItem
)

func (a *App) startTray() {
	start, _ := systray.RunWithExternalLoop(a.onTrayReady, func() {})
	start()
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTitle("Deliverance")
	systray.SetTooltip("Deliverance")
	systray.SetOnTapped(a.showWindow)

	mOpen := systray.AddMenuItem("Открыть", "")
	mToggle = systray.AddMenuItem("Запустить", "")
	mTg = systray.AddMenuItem("Прокси для Telegram: выкл", "")
	mTgOpen := systray.AddMenuItem("Подключить Telegram", "Добавить прокси в Telegram")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Выход", "Остановить zapret и закрыть программу")

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				a.showWindow()
			case <-mToggle.ClickedCh:
				if isRunning() {
					go a.Stop()
				} else {
					go a.Start()
				}
			case <-mTg.ClickedCh:
				go a.SetTgProxy(!a.tg.Running())
			case <-mTgOpen.ClickedCh:
				go a.OpenTgLink()
			case <-mQuit.ClickedCh:
				a.QuitApp()
				return
			}
		}
	}()
	a.updateTray()
}

func (a *App) updateTray() {
	if mToggle == nil {
		return
	}
	if mTg != nil && a.tg != nil {
		if a.tg.Running() {
			mTg.SetTitle("Прокси для Telegram: вкл")
		} else {
			mTg.SetTitle("Прокси для Telegram: выкл")
		}
	}
	if isRunning() {
		mToggle.SetTitle("Остановить")
		systray.SetTooltip("Deliverance — работает")
	} else {
		mToggle.SetTitle("Запустить")
		systray.SetTooltip("Deliverance — остановлено")
	}
}

func (a *App) showWindow() {
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

// QuitApp — полный выход: останавливает zapret и закрывает программу.
func (a *App) QuitApp() {
	stopZapret()
	if a.tg != nil {
		a.tg.Stop()
	}
	systray.Quit()
	runtime.Quit(a.ctx)
}
