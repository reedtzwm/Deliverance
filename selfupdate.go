package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"zapretgui/internal/updsig"
)

// ---- ЗАГОТОВКА самообновления самого приложения ----
//
// Чтобы включить: впишите свой репозиторий в selfRepo ("логин/репозиторий")
// и публикуйте в Releases установщик с именем Deliverance-Setup-<версия>.exe
// (тег релиза — например 1.1.0 или v1.1.0).
// Версию при сборке можно задать так: go build -ldflags "-X main.appVersion=1.1.0"

var appVersion = "1.7.1"

const selfRepo = "" // TODO: "ВАШ_ЛОГИН/ВАШ_РЕПОЗИТОРИЙ"

// Префикс имени установщика в релизе: <префикс><версия>.exe и рядом <то же>.exe.sig
const installerPrefix = "Deliverance-Setup-"

// Публичный ключ для проверки обновлений. Парный приватный ключ лежит только у автора
// (%USERPROFILE%\.zapretgui-signing\update.key) и в репозиторий не попадает.
// Обновления без валидной подписи этим ключом отклоняются.
const updatePubKeyHex = "c6eaf0766aeaed254a4139026480eaa6bd952b43afa9e6e5e787c7316004613f"

type SelfUpdateInfo struct {
	Configured bool   `json:"configured"`
	Current    string `json:"current"`
	Latest     string `json:"latest"`
	Tag        string `json:"-"` // исходный тег релиза (может быть с «v»)
	Available  bool   `json:"available"`
	Error      string `json:"error"`
}

// CheckSelfUpdate — только проверка, ничего не скачивает.
func (a *App) CheckSelfUpdate() SelfUpdateInfo {
	info := SelfUpdateInfo{Configured: selfRepo != "", Current: appVersion}
	if !info.Configured {
		return info
	}
	ctx, cancel := context.WithTimeout(a.ctx, 25*time.Second)
	defer cancel()
	tag, err := latestTag(ctx, selfRepo)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	info.Tag = tag
	info.Latest = strings.TrimPrefix(tag, "v")
	info.Available = versionLess(appVersion, info.Latest)
	return info
}

// ApplySelfUpdate скачивает установщик новой версии и запускает его тихо.
// Установщик сам закроет приложение (CloseApplications) и поставит новую версию поверх.
func (a *App) ApplySelfUpdate() string {
	info := a.CheckSelfUpdate()
	if !info.Configured {
		return "Самообновление не настроено (selfRepo пуст)"
	}
	if !info.Available {
		return "Установлена последняя версия"
	}
	// имя файла строится из версии без «v», каталог загрузки — из исходного тега
	name := fmt.Sprintf("%s%s.exe", installerPrefix, info.Latest)
	base := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", selfRepo, info.Tag, name)
	dst := filepath.Join(os.TempDir(), name)
	_ = os.Remove(dst)

	if err := download(base, dst, 200<<20); err != nil {
		return "Ошибка загрузки: " + err.Error()
	}
	sigBytes, err := fetchSmall(base+".sig", 4096)
	if err != nil {
		_ = os.Remove(dst)
		return "Не удалось получить подпись обновления: " + err.Error()
	}
	// запуск только после успешной проверки подписи
	if err := updsig.VerifyFile(updatePubKeyHex, string(sigBytes), dst); err != nil {
		_ = os.Remove(dst)
		return "Обновление отклонено: " + err.Error()
	}
	if err := exec.Command(dst, "/VERYSILENT", "/SUPPRESSMSGBOXES", "/CLOSEAPPLICATIONS", "/RESTARTAPPLICATIONS").Start(); err != nil {
		return err.Error()
	}
	return "Подпись проверена, установка обновления запущена"
}

func download(url, dst string, limit int64) error {
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
		return fmt.Errorf("файл слишком большой")
	}
	return nil
}

func fetchSmall(url string, limit int64) ([]byte, error) {
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// versionLess сравнивает версии вида 1.2.3.
func versionLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		x, y := 0, 0
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x < y
		}
	}
	return false
}
