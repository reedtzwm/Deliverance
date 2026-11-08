package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Диагностика: проверяет доступность сервисов по отдельности и объясняет, что видно.

type DiagItem struct {
	Group  string `json:"group"`
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Ms     int64  `json:"ms"`
	Reason string `json:"reason"` // "" | dns | timeout | reset | refused | tls | other
	Kind   string `json:"kind"`   // https | tcp | ws
}

type diagTarget struct {
	Group, Name, Kind, Addr string
}

func diagTargets() []diagTarget {
	var t []diagTarget
	https := func(group, host string) {
		t = append(t, diagTarget{group, host, "https", "https://" + host})
	}
	for _, h := range []string{"discord.com", "gateway.discord.gg", "cdn.discordapp.com"} {
		https("Discord", h)
	}
	for _, h := range []string{"www.youtube.com", "i.ytimg.com", "redirector.googlevideo.com"} {
		https("YouTube", h)
	}
	for _, h := range []string{"web.telegram.org", "api.telegram.org", "kws2.web.telegram.org"} {
		https("Telegram", h)
	}
	// прямые адреса дата-центров Telegram (по ним работает приложение)
	var dcs []int
	for dc := range dcDefaultIP {
		if dc <= 5 {
			dcs = append(dcs, dc)
		}
	}
	sort.Ints(dcs)
	for _, dc := range dcs {
		ip := dcDefaultIP[dc]
		t = append(t, diagTarget{"Telegram", fmt.Sprintf("DC%d %s", dc, ip), "tcp", net.JoinHostPort(ip, "443")})
	}
	t = append(t, diagTarget{"Telegram", "Маршрут встроенного прокси (WebSocket)", "ws", ""})
	https("Google и Cloudflare", "www.google.com")
	https("Google и Cloudflare", "www.cloudflare.com")

	// домены пользователя (игры и т.д.), не больше 12
	good, _ := normalizeDomains(loadConfig().CustomDomains)
	for i, d := range good {
		if i >= 12 {
			break
		}
		https("Ваши домены", d)
	}
	return t
}

// classify определяет тип сбоя по тексту ошибки (Windows) и её типу.
func classify(err error) string {
	if err == nil {
		return ""
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && !dnsErr.IsTimeout {
		return "dns"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "timeout"
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "no such host"), strings.Contains(s, "server misbehaving"):
		return "dns"
	case strings.Contains(s, "connection reset"), strings.Contains(s, "forcibly closed"), strings.Contains(s, "wsarecv"), strings.Contains(s, "eof"):
		return "reset"
	case strings.Contains(s, "refused"), strings.Contains(s, "actively refused"):
		return "refused"
	case strings.Contains(s, "timeout"), strings.Contains(s, "timed out"), strings.Contains(s, "deadline"):
		return "timeout"
	case strings.Contains(s, "tls"), strings.Contains(s, "certificate"), strings.Contains(s, "handshake"):
		return "tls"
	}
	return "other"
}

func probeHTTPS(ctx context.Context, url string) (int64, error) {
	tr := &http.Transport{
		DisableKeepAlives:   true,
		TLSHandshakeTimeout: 7 * time.Second,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext:         (&net.Dialer{Timeout: 6 * time.Second}).DialContext,
	}
	defer tr.CloseIdleConnections()
	client := &http.Client{
		Transport:     tr,
		Timeout:       9 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Deliverance")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return time.Since(start).Milliseconds(), nil
}

func probeTCP(ctx context.Context, addr string) (int64, error) {
	start := time.Now()
	c, err := (&net.Dialer{Timeout: 6 * time.Second}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return 0, err
	}
	c.Close()
	return time.Since(start).Milliseconds(), nil
}

// probeWS проверяет тот самый маршрут, которым пользуется встроенный Telegram-прокси.
func probeWS() (int64, error) {
	start := time.Now()
	c := dialWS(2, false, false)
	if c == nil {
		return 0, errors.New("timeout: WebSocket-шлюз Telegram недоступен")
	}
	c.Close()
	return time.Since(start).Milliseconds(), nil
}

// retryOnDNS повторяет проверку, если она упала на получении адреса: при параллельных
// запросах системный DNS (особенно у виртуальных адаптеров VPN/Xray) иногда отвечает
// сбоем, хотя имя на самом деле существует. Остальные ошибки не повторяются — они
// и так показывают блокировку, а повтор только удлинил бы проверку.
func retryOnDNS(fn func() (int64, error)) (int64, error) {
	delays := []time.Duration{0, 400 * time.Millisecond, 900 * time.Millisecond}
	var ms int64
	var err error
	for _, d := range delays {
		time.Sleep(d)
		ms, err = fn()
		if err == nil || classify(err) != "dns" {
			return ms, err
		}
	}
	return ms, err
}

func runProbe(ctx context.Context, t diagTarget) DiagItem {
	var ms int64
	var err error
	switch t.Kind {
	case "https":
		ms, err = retryOnDNS(func() (int64, error) { return probeHTTPS(ctx, t.Addr) })
	case "tcp":
		ms, err = probeTCP(ctx, t.Addr)
	case "ws":
		ms, err = probeWS()
	}
	return DiagItem{Group: t.Group, Name: t.Name, Kind: t.Kind, OK: err == nil, Ms: ms, Reason: classify(err)}
}

// diagHints — выводы простым языком по результатам проверки.
func diagHints(items []DiagItem, bypassOn bool) []string {
	var dns, reset, httpsTimeout, tcpTimeout, tls, fails, httpsOK int
	for _, it := range items {
		if it.OK {
			if it.Kind == "https" {
				httpsOK++
			}
			continue
		}
		fails++
		switch it.Reason {
		case "dns":
			dns++
		case "reset":
			reset++
		case "tls":
			tls++
		case "timeout":
			if it.Kind == "tcp" || it.Kind == "ws" {
				tcpTimeout++
			} else {
				httpsTimeout++
			}
		}
	}
	var h []string
	if fails == 0 {
		h = append(h, "Всё проверенное доступно.")
		if !bypassOn {
			h = append(h, "Обход выключен — значит, эти сервисы у вас сейчас не заблокированы.")
		}
		return h
	}
	if dns > 0 {
		h = append(h, fmt.Sprintf("Не удалось получить адрес (DNS) у %d из проверок. Похоже на блокировку или подмену DNS: смените DNS в Windows, например на 1.1.1.1 или 8.8.8.8.", dns))
	}
	if reset+tls > 0 {
		s := fmt.Sprintf("%d соединений оборваны сразу после начала — типичный признак DPI-блокировки по имени сайта. ", reset+tls)
		if bypassOn {
			s += "Обход включён, но сбросы остались: нажмите «Подобрать автоматически» на главной."
		} else {
			s += "Обход (zapret) обычно с этим справляется: запустите его на главной."
		}
		h = append(h, s)
	}
	if tcpTimeout > 0 && httpsOK > 0 {
		h = append(h, fmt.Sprintf("%d адресов не отвечают вовсе (таймаут), хотя другие сайты открываются. Это похоже на блокировку по IP-адресу: zapret здесь не поможет, нужен прокси или туннель.", tcpTimeout))
	}
	if httpsTimeout > 0 {
		h = append(h, fmt.Sprintf("%d сайтов не ответили вовремя. Это может быть блокировка, а может — просто медленная сеть; проверьте ещё раз.", httpsTimeout))
	}
	if len(h) == 0 {
		h = append(h, "Часть проверок не прошла по неясной причине. Повторите проверку.")
	}
	return h
}

var diagMu sync.Mutex
var diagRunning bool

// RunDiagnostics запускает проверку; результаты приходят событиями «diag».
func (a *App) RunDiagnostics() {
	diagMu.Lock()
	if diagRunning {
		diagMu.Unlock()
		return
	}
	diagRunning = true
	diagMu.Unlock()

	go func() {
		defer func() { diagMu.Lock(); diagRunning = false; diagMu.Unlock() }()
		targets := diagTargets()
		bypassOn := isRunning()
		runtime.EventsEmit(a.ctx, "diag", map[string]any{
			"type": "start", "bypass": bypassOn, "tg": a.tg != nil && a.tg.Running(), "total": len(targets),
		})

		ctx, cancel := context.WithTimeout(a.ctx, 25*time.Second)
		defer cancel()
		var mu sync.Mutex
		var items []DiagItem
		sem := make(chan struct{}, 8)
		var wg sync.WaitGroup
		for _, t := range targets {
			wg.Add(1)
			sem <- struct{}{}
			go func(t diagTarget) {
				defer wg.Done()
				defer func() { <-sem }()
				it := runProbe(ctx, t)
				mu.Lock()
				items = append(items, it)
				mu.Unlock()
				runtime.EventsEmit(a.ctx, "diag", map[string]any{"type": "item", "item": it})
			}(t)
		}
		wg.Wait()
		runtime.EventsEmit(a.ctx, "diag", map[string]any{"type": "done", "hints": diagHints(items, bypassOn)})
	}()
}
