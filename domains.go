package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Пресеты — примерные наборы доменов для популярных игр и лаунчеров.
// hostlist в zapret сопоставляет и поддомены, поэтому достаточно базовых доменов.
// Это отправная точка: если игра не заработала, домены можно дописать вручную.
type Preset struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Domains []string `json:"domains"`
}

var steamDomains = []string{"steampowered.com", "steamcommunity.com", "steamstatic.com", "steamcontent.com", "steam-chat.com", "steamserver.net"}
var eacDomains = []string{"easyanticheat.net", "eac-cdn.com"}

// Photon — сетевой движок, на котором работают многие кооперативные игры.
var photonDomains = []string{"photonengine.io", "exitgames.com"}

func presets() []Preset {
	join := func(parts ...[]string) []string {
		var r []string
		for _, p := range parts {
			r = append(r, p...)
		}
		return r
	}
	return []Preset{
		{"steam", "Steam", steamDomains},
		{"outlast", "The Outlast Trials", join([]string{"theoutlasttrials.com", "redbarrelsgames.com"}, steamDomains, eacDomains)},
		{"awayout", "A Way Out", []string{"ea.com", "origin.com", "hazelight.se"}},
		{"ea", "EA app / EA Games", []string{"ea.com", "origin.com", "easports.com"}},
		{"epic", "Epic Games / Fortnite", join([]string{"epicgames.com", "epicgames.dev", "unrealengine.com", "fortnite.com"}, eacDomains)},
		{"ubisoft", "Ubisoft Connect", []string{"ubisoft.com", "ubi.com", "ubisoftconnect.com"}},
		{"blizzard", "Battle.net / Blizzard", []string{"battle.net", "blizzard.com"}},
		{"riot", "Riot Games (LoL, Valorant)", []string{"riotgames.com", "leagueoflegends.com", "playvalorant.com", "pvp.net"}},
		{"rockstar", "Rockstar (GTA Online)", []string{"rockstargames.com"}},
		{"minecraft", "Minecraft", []string{"minecraft.net", "mojang.com"}},
		{"roblox", "Roblox", []string{"roblox.com", "rbxcdn.com"}},
		{"tarkov", "Escape from Tarkov", []string{"escapefromtarkov.com", "battlestategames.com"}},
		{"hoyo", "Genshin / HoYoverse", []string{"hoyoverse.com", "mihoyo.com", "hoyolab.com"}},
		{"dbd", "Dead by Daylight", join([]string{"deadbydaylight.com", "bhvr.com"}, steamDomains, eacDomains)},
		{"rust", "Rust", join([]string{"facepunch.com", "rust.facepunch.com"}, steamDomains, eacDomains)},
		{"warframe", "Warframe", []string{"warframe.com"}},
		{"repo", "R.E.P.O.", join(steamDomains, photonDomains)},
		{"peak", "PEAK", join(steamDomains, photonDomains)},
		{"stumble", "Stumble Guys", join([]string{"stumbleguys.com", "kitkagames.com", "scopely.com"}, photonDomains)},
		{"phasmo", "Phasmophobia", join([]string{"kineticgames.co.uk"}, steamDomains, photonDomains)},
		{"albion", "Albion Online", join([]string{"albiononline.com", "albion-online-data.com", "sandboxinteractive.com"}, steamDomains)},
		{"vrchat", "VRChat", join([]string{"vrchat.com", "vrchat.cloud", "vrcdn.cloud", "vrcdn.video"}, steamDomains, photonDomains, eacDomains)},
		{"cwarning", "Content Warning", join([]string{"landfall.se"}, steamDomains)},
		{"apex", "Apex Legends", join([]string{"apexlegends.com", "ea.com", "origin.com", "respawn.com"}, steamDomains, eacDomains)},
		{"cs2dota", "Counter-Strike 2 / Dota 2", join([]string{"counter-strike.net", "dota2.com"}, steamDomains)},
	}
}

func (a *App) GetPresets() []Preset { return presets() }

var domainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// normalizeDomains приводит ввод пользователя к списку доменов:
// принимает адреса целиком (https://host/путь), разделители — пробел, запятая, новая строка.
func normalizeDomains(text string) (good []string, bad []string) {
	seen := map[string]bool{}
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			kept = append(kept, line)
		}
	}
	fields := strings.FieldsFunc(strings.Join(kept, "\n"), func(r rune) bool { return r == '\n' || r == '\r' || r == ',' || r == ';' || r == ' ' || r == '\t' })
	for _, f := range fields {
		d := strings.ToLower(strings.TrimSpace(f))
		if d == "" {
			continue
		}
		if i := strings.Index(d, "://"); i >= 0 {
			d = d[i+3:]
		}
		if i := strings.IndexAny(d, "/:?#"); i >= 0 {
			d = d[:i]
		}
		d = strings.TrimPrefix(d, "www.")
		d = strings.Trim(d, ".")
		if !domainRe.MatchString(d) {
			bad = append(bad, f)
			continue
		}
		if !seen[d] {
			seen[d] = true
			good = append(good, d)
		}
	}
	return
}

const (
	blockStart = "# >>> Deliverance (управляется программой, не редактируйте между метками)"
	blockEnd   = "# <<< Deliverance"

	// метки прежней версии: при следующем применении блок будет заменён на новый
	legacyBlockStart = "# >>> ZapretGUI (управляется программой, не редактируйте между метками)"
	legacyBlockEnd   = "# <<< ZapretGUI"
)

// applyDomains записывает домены в lists/list-general-user.txt внутри своего блока,
// не трогая остальное содержимое файла.
func applyDomains(text string) error {
	path := filepath.Join(zapretDir(), "lists", "list-general-user.txt")
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		return nil // zapret ещё не установлен — применится после установки
	}
	existing := ""
	if b, err := os.ReadFile(path); err == nil {
		existing = string(b)
	}
	// вырезаем старый блок (и блок с метками прежней версии)
	for _, m := range [][2]string{{blockStart, blockEnd}, {legacyBlockStart, legacyBlockEnd}} {
		if i := strings.Index(existing, m[0]); i >= 0 {
			if j := strings.Index(existing[i:], m[1]); j >= 0 {
				existing = existing[:i] + existing[i+j+len(m[1]):]
			}
		}
	}
	// единый перевод строк: иначе при каждом применении «\r\n» превращался в «\r\r\n»
	existing = strings.ReplaceAll(strings.ReplaceAll(existing, "\r\n", "\n"), "\r", "\n")
	existing = strings.TrimRight(existing, "\n ")
	if existing == "" {
		existing = "# Never leave this file empty\ndomain.example.abc"
	}
	good, _ := normalizeDomains(text)
	var sb strings.Builder
	sb.WriteString(existing)
	sb.WriteString("\n")
	if len(good) > 0 {
		sb.WriteString(blockStart + "\n")
		for _, d := range good {
			sb.WriteString(d + "\n")
		}
		sb.WriteString(blockEnd + "\n")
	}
	return os.WriteFile(path, []byte(strings.ReplaceAll(sb.String(), "\n", "\r\n")), 0o644)
}

// applyGameFilter: off | all | tcp | udp — файл utils/game_filter.enabled, который читает service.bat.
func applyGameFilter(mode string) error {
	path := filepath.Join(zapretDir(), "utils", "game_filter.enabled")
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		return nil
	}
	switch mode {
	case "all", "tcp", "udp":
		return os.WriteFile(path, []byte("mode="+mode+"\r\n"), 0o644)
	default:
		_ = os.Remove(path)
		return nil
	}
}

// applyUserSettings вызывается перед каждым запуском батника — так настройки
// сохраняются и после обновления файлов zapret.
func applyUserSettings() {
	c := loadConfig()
	_ = applyDomains(c.CustomDomains)
	_ = applyGameFilter(c.GameFilter)
}

// SetDomains сохраняет список доменов и режим игрового фильтра, перезапускает zapret при необходимости.
// Возвращает сообщение для пользователя.
func (a *App) SetDomains(text string, gameFilter string) string {
	good, bad := normalizeDomains(text)
	c := loadConfig()
	c.CustomDomains = strings.Join(good, "\n")
	c.GameFilter = gameFilter
	if err := saveConfig(c); err != nil {
		return "Не удалось сохранить: " + err.Error()
	}
	applyUserSettings()
	msg := "Сохранено доменов: " + itoa(len(good))
	if len(bad) > 0 {
		msg += ". Пропущено как некорректные: " + strings.Join(bad, ", ")
	}
	a.logf(msg)
	a.emitState()
	if isRunning() && c.SelectedBat != "" {
		go func() {
			if _, ok := a.begin(); !ok {
				return
			}
			defer a.end()
			a.logf("Перезапускаю zapret, чтобы применить списки")
			if err := startBat(c.SelectedBat); err != nil {
				a.logf("Ошибка перезапуска: " + err.Error())
			}
		}()
	}
	return msg
}
