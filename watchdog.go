package main

import "time"

const watchdogInterval = 10 * time.Minute

// watchdogLoop: пока включён тумблер, раз в 10 минут проверяет доступность целей.
// Если большинство недоступно (дважды подряд) — заново подбирает конфигурацию.
func (a *App) watchdogLoop() {
	t := time.NewTicker(watchdogInterval)
	defer t.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-t.C:
		}
		if !loadConfig().Watchdog || !isRunning() {
			continue
		}
		a.mu.Lock()
		busy := a.busy
		a.mu.Unlock()
		if busy {
			continue
		}
		if a.healthy() {
			continue
		}
		a.logf("Проверка: часть сервисов недоступна, повторяю…")
		time.Sleep(20 * time.Second)
		if a.healthy() {
			continue
		}
		a.logf("Проверка не пройдена — запускаю повторный подбор")
		a.AutoSelect()
	}
}

func (a *App) healthy() bool {
	ok, total := score(a.ctx, loadTargets())
	return total == 0 || ok*2 >= total
}
