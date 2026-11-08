package main

import (
	"context"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) AppVersion() string { return appVersion }

type State struct {
	Version   string   `json:"version"`
	Running   bool     `json:"running"`
	Selected  string   `json:"selected"`
	Bats      []string `json:"bats"`
	Busy      bool     `json:"busy"`
	Config    Config   `json:"config"`
	TgRunning bool     `json:"tgRunning"`
	TgLink    string   `json:"tgLink"`
}

type App struct {
	tg     *TgProxy
	ctx    context.Context
	mu     sync.Mutex
	busy   bool
	cancel context.CancelFunc
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.tg = NewTgProxy(a.logf)
	if c := loadConfig(); c.TgProxy {
		a.startTg()
	}
	go a.startTray()
	go a.watchdogLoop()
	go a.boot()
}

func (a *App) logf(s string) {
	runtime.EventsEmit(a.ctx, "log", time.Now().Format("15:04:05")+"  "+s)
}

func (a *App) emitState() {
	runtime.EventsEmit(a.ctx, "state", a.GetState())
	a.updateTray()
}

func (a *App) GetState() State {
	a.mu.Lock()
	busy := a.busy
	a.mu.Unlock()
	c := loadConfig()
	return State{
		Version:   installedVersion(),
		Running:   isRunning(),
		Selected:  c.SelectedBat,
		Bats:      listBats(),
		Busy:      busy,
		Config:    c,
		TgRunning: a.tg != nil && a.tg.Running(),
		TgLink:    tgLink(c.TgSecret),
	}
}

// begin/end — простая блокировка, чтобы операции не шли параллельно.
func (a *App) begin() (context.Context, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.busy {
		return nil, false
	}
	a.busy = true
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	return ctx, true
}

func (a *App) end() {
	a.mu.Lock()
	a.busy = false
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()
	a.emitState()
}

// boot: при старте — обновление, затем запуск/подбор.
func (a *App) boot() {
	migrateLegacy()
	cfg := loadConfig()
	if installedVersion() == "" || cfg.AutoUpdate {
		a.CheckUpdate()
	}
	if cfg.AutoStart && !isRunning() {
		a.Start()
	}
	a.emitState()
}

// CheckUpdate проверяет GitHub и при необходимости обновляет файлы zapret.
func (a *App) CheckUpdate() {
	ctx, ok := a.begin()
	if !ok {
		return
	}
	defer a.end()
	a.emitState()

	tag, err := latestTag(ctx, repo)
	if err != nil {
		a.logf("Не удалось проверить обновления: " + err.Error())
		return
	}
	cur := installedVersion()
	if cur == tag {
		a.logf("Версия актуальна: " + tag)
		return
	}
	a.logf("Новая версия: " + tag + " (установлена: " + orDash(cur) + ")")
	wasRunning := isRunning()
	stopZapret()
	if err := installRelease(ctx, tag, a.logf); err != nil {
		a.logf("Ошибка обновления: " + err.Error())
		return
	}
	if wasRunning {
		c := loadConfig()
		if c.SelectedBat != "" {
			_ = startBat(c.SelectedBat)
		}
	}
}

// Start запускает выбранный батник; если не выбран — подбирает.
func (a *App) Start() {
	c := loadConfig()
	if c.SelectedBat == "" || !contains(listBats(), c.SelectedBat) {
		a.AutoSelect()
		return
	}
	ctx, ok := a.begin()
	if !ok {
		return
	}
	defer a.end()
	_ = ctx
	a.logf("Запускаю " + c.SelectedBat)
	if err := startBat(c.SelectedBat); err != nil {
		a.logf("Ошибка запуска: " + err.Error())
		return
	}
	a.logf("Запущено")
}

func (a *App) Stop() {
	stopZapret()
	a.logf("Остановлено")
	a.emitState()
}

// AutoSelect перебирает все батники и оставляет рабочий.
func (a *App) AutoSelect() {
	ctx, ok := a.begin()
	if !ok {
		return
	}
	defer a.end()
	a.emitState()

	bats := listBats()
	if len(bats) == 0 {
		a.logf("Батники не найдены. Нажмите «Проверить обновления».")
		return
	}
	// сначала пробуем ранее выбранный
	c := loadConfig()
	if c.SelectedBat != "" && contains(bats, c.SelectedBat) {
		rest := []string{c.SelectedBat}
		for _, b := range bats {
			if b != c.SelectedBat {
				rest = append(rest, b)
			}
		}
		bats = rest
	}
	name, good := selectBest(ctx, bats, a.logf, func(i, n int, nm string) {
		runtime.EventsEmit(a.ctx, "progress", map[string]any{"i": i, "n": n, "name": nm})
	})
	runtime.EventsEmit(a.ctx, "progress", map[string]any{"i": 0, "n": 0, "name": ""})
	if name == "" {
		a.logf("Рабочий вариант не найден")
		return
	}
	c.SelectedBat = name
	_ = saveConfig(c)
	if good {
		a.logf("Выбран и запущен: " + name)
	} else {
		a.logf("Запущен лучший из найденных (может работать частично): " + name)
	}
}

// SelectBat — ручной выбор батника.
func (a *App) SelectBat(name string) {
	c := loadConfig()
	c.SelectedBat = name
	_ = saveConfig(c)
	a.emitState()
}

func (a *App) SetConfig(c Config) {
	cur := loadConfig()
	c.SelectedBat = cur.SelectedBat
	c.CustomDomains = cur.CustomDomains
	c.GameFilter = cur.GameFilter
	c.TgProxy = cur.TgProxy
	c.TgSecret = cur.TgSecret
	c.Theme = cur.Theme
	_ = saveConfig(c)
	setAutostart(c.AutoRun)
	a.emitState()
}

// CancelOperation прерывает текущий подбор/загрузку.
func (a *App) CancelOperation() {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()
}

func (a *App) shutdown(ctx context.Context) {}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
