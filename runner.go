package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const hideWindow = 0x08000000 // CREATE_NO_WINDOW

func hidden(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: hideWindow}
	return cmd
}

// listBats возвращает имена general*.bat (service.bat исключён).
func listBats() []string {
	matches, _ := filepath.Glob(filepath.Join(zapretDir(), "general*.bat"))
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, filepath.Base(m))
	}
	sort.SliceStable(names, func(i, j int) bool {
		// general.bat первым, остальные по алфавиту
		if names[i] == "general.bat" {
			return true
		}
		if names[j] == "general.bat" {
			return false
		}
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	return names
}

// batCommand собирает запуск батника скрытым окном.
//
// Имена вида "general (ALT).bat" содержат пробелы и скобки. При обычной форме
// `cmd /c "имя"` cmd отбрасывает кавычки и пытается запустить "general" с параметром
// "(ALT).bat" — батник не стартует. Поэтому используем /s и берём весь путь в двойные
// кавычки: `cmd /d /s /c ""путь""`. Полный путь заодно не зависит от текущей папки.
func batCommand(dir, name string) *exec.Cmd {
	cmd := exec.Command("cmd.exe")
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: hideWindow,
		CmdLine:       `cmd.exe /d /s /c ""` + filepath.Join(dir, name) + `""`,
	}
	return cmd
}

func isRunning() bool {
	out, err := hidden(exec.Command("tasklist", "/FI", "IMAGENAME eq winws.exe", "/NH")).Output()
	return err == nil && strings.Contains(strings.ToLower(string(out)), "winws.exe")
}

func stopZapret() {
	_ = hidden(exec.Command("taskkill", "/F", "/IM", "winws.exe")).Run()
	// драйвер WinDivert остаётся загруженным — освобождаем, чтобы можно было обновить файлы
	_ = hidden(exec.Command("sc", "stop", "WinDivert")).Run()
	_ = hidden(exec.Command("sc", "stop", "WinDivert14")).Run()
	for i := 0; i < 20 && isRunning(); i++ {
		time.Sleep(150 * time.Millisecond)
	}
}

// startBat запускает батник скрытно. Сам батник через `start /min` порождает winws.exe и завершается.
func startBat(name string) error {
	stopZapret()
	applyUserSettings()
	cmd := batCommand(zapretDir(), name)
	cmd.Stdin = nil // NUL — чтобы `pause` в батнике не вешал процесс
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	// ждём появления winws.exe
	for i := 0; i < 40; i++ {
		if isRunning() {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return os.ErrNotExist
}
