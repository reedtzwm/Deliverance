package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type target struct {
	Name string
	URL  string
}

var targetRe = regexp.MustCompile(`^\s*(\w+)\s*=\s*"(https?://[^"]+)"`)

// loadTargets читает utils/targets.txt из релиза (только https-цели).
func loadTargets() []target {
	var res []target
	if b, err := os.ReadFile(filepath.Join(zapretDir(), "utils", "targets.txt")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if m := targetRe.FindStringSubmatch(line); m != nil {
				res = append(res, target{m[1], m[2]})
			}
		}
	}
	if len(res) == 0 {
		res = []target{
			{"DiscordMain", "https://discord.com"},
			{"DiscordGateway", "https://gateway.discord.gg"},
			{"DiscordCDN", "https://cdn.discordapp.com"},
			{"YouTubeWeb", "https://www.youtube.com"},
			{"YouTubeImage", "https://i.ytimg.com"},
			{"YouTubeVideo", "https://redirector.googlevideo.com"},
		}
	}
	return res
}

// probe: цель считается доступной, если TLS-рукопожатие и HTTP-ответ прошли (любой код).
func probe(ctx context.Context, t target) bool {
	tr := &http.Transport{
		DisableKeepAlives:   true,
		TLSHandshakeTimeout: 6 * time.Second,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		Proxy:               nil,
	}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 8 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 DZapret")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// score — число доступных целей и общее их количество.
func score(ctx context.Context, targets []target) (ok, total int) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, t := range targets {
		wg.Add(1)
		go func(t target) {
			defer wg.Done()
			if probe(ctx, t) {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}(t)
	}
	wg.Wait()
	return ok, len(targets)
}

// selectBest перебирает батники. Возвращает имя лучшего (запущенного) и его результат.
// Останавливается на первом, где доступны все цели; иначе остаётся лучший по баллам.
func selectBest(ctx context.Context, bats []string, logf func(string), progress func(i, n int, name string)) (string, bool) {
	targets := loadTargets()
	best, bestOK := "", -1
	for i, name := range bats {
		if ctx.Err() != nil {
			break
		}
		progress(i+1, len(bats), name)
		logf("Проверяю " + name)
		if err := startBat(name); err != nil {
			logf("  не удалось запустить: " + err.Error())
			continue
		}
		time.Sleep(3 * time.Second) // дать winws инициализироваться
		ok, total := score(ctx, targets)
		logf("  доступно " + itoa(ok) + " из " + itoa(total))
		if ok > bestOK {
			best, bestOK = name, ok
		}
		if ok == total {
			return name, true
		}
	}
	stopZapret()
	if best == "" {
		return "", false
	}
	// минимум порога: хотя бы 70 % целей
	if float64(bestOK) < 0.7*float64(len(targets)) {
		logf("Ни один вариант не прошёл порог; лучший: " + best)
	}
	if err := startBat(best); err != nil {
		return "", false
	}
	return best, bestOK > 0
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
