package main

// Локальный MTProto-прокси для Telegram Desktop: клиент подключается к 127.0.0.1,
// а трафик уходит к серверам Telegram через WebSocket (kwsN.web.telegram.org).
//
// Это порт на Go ядра проекта Flowseal/tg-ws-proxy (лицензия MIT):
// https://github.com/Flowseal/tg-ws-proxy
// Портировано: рукопожатие obfuscated2, перешифровка потока, WS-туннель (обычный и
// «fronting» с другим SNI), запасной прямой TCP. Не портировано: Fake TLS,
// Cloudflare-прокси и HTTP/2-медиа. Текст лицензии — licenses\tg-ws-proxy-LICENSE.txt.

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	hsLen       = 64
	skipLen     = 8
	prekeyLen   = 32
	ivLen       = 16
	protoTagPos = 56
	dcIdxPos    = 60

	protoAbridged     uint32 = 0xEFEFEFEF
	protoIntermediate uint32 = 0xEEEEEEEE
	protoPadded       uint32 = 0xDDDDDDDD

	tgDefaultPort = 1443
	wsPath        = "/apiws"
	frontingSNI   = "sprinthost.ru"
)

var (
	tagAbridged     = [4]byte{0xef, 0xef, 0xef, 0xef}
	tagIntermediate = [4]byte{0xee, 0xee, 0xee, 0xee}
	tagSecure       = [4]byte{0xdd, 0xdd, 0xdd, 0xdd}
)

// IP «входов» WebSocket по умолчанию (как в оригинале) и прямые адреса DC для запасного TCP.
var (
	dcRedirects = map[int]string{2: "149.154.167.220", 4: "149.154.167.220"}
	dcDefaultIP = map[int]string{1: "149.154.175.50", 2: "149.154.167.51", 3: "149.154.175.100",
		4: "149.154.167.91", 5: "149.154.171.5", 203: "91.105.192.100"}
	dcTestIP = map[int]string{1: "149.154.175.10", 2: "149.154.167.40", 3: "149.154.175.117"}
)

// Точки подмены для тестов.
var (
	wsDialAddr    = func(ip string) string { return net.JoinHostPort(ip, "443") }
	wsInsecureAll = false
)

func wsDomains(dc int, media bool) []string {
	if dc == 203 {
		dc = 2
	}
	if !media {
		return []string{fmt.Sprintf("kws%d.web.telegram.org", dc)}
	}
	return []string{fmt.Sprintf("kws%d-1.web.telegram.org", dc), fmt.Sprintf("kws%d.web.telegram.org", dc)}
}

// ---------- криптография obfuscated2 ----------

func newCTR(key, iv []byte) cipher.Stream {
	b, _ := aes.NewCipher(key)
	return cipher.NewCTR(b, iv)
}

func skip(s cipher.Stream, n int) {
	buf := make([]byte, n)
	s.XORKeyStream(buf, buf)
}

func reversed(b []byte) []byte {
	r := make([]byte, len(b))
	for i := range b {
		r[len(b)-1-i] = b[i]
	}
	return r
}

func keyFor(prekey, secret []byte) []byte {
	h := sha256.New()
	h.Write(prekey)
	h.Write(secret)
	return h.Sum(nil)
}

type cryptoCtx struct {
	cltDec, cltEnc, tgEnc, tgDec cipher.Stream
}

// tryHandshake разбирает 64 байта от клиента. Возвращает DC, признак media, тег протокола
// и 48 байт prekey+iv; ok=false при неверном секрете или протоколе.
func tryHandshake(hs, secret []byte) (dc int, media bool, tag [4]byte, prekeyIV []byte, ok bool) {
	prekeyIV = hs[skipLen : skipLen+prekeyLen+ivLen]
	dec := make([]byte, hsLen)
	newCTR(keyFor(prekeyIV[:prekeyLen], secret), prekeyIV[prekeyLen:]).XORKeyStream(dec, hs)
	copy(tag[:], dec[protoTagPos:protoTagPos+4])
	if tag != tagAbridged && tag != tagIntermediate && tag != tagSecure {
		return 0, false, tag, nil, false
	}
	idx := int(int16(binary.LittleEndian.Uint16(dec[dcIdxPos:])))
	if idx < 0 {
		return -idx, true, tag, prekeyIV, true
	}
	return idx, false, tag, prekeyIV, true
}

func reservedStart(b []byte) bool {
	s := string(b[:4])
	switch s {
	case "HEAD", "POST", "GET ", "OPTI", "\xee\xee\xee\xee", "\xdd\xdd\xdd\xdd", "\x16\x03\x01\x02":
		return true
	}
	return false
}

// genRelayInit создаёт 64-байтовое рукопожатие к серверу Telegram с нужными DC и протоколом.
func genRelayInit(tag [4]byte, dcIdx int) []byte {
	for {
		rnd := make([]byte, hsLen)
		_, _ = rand.Read(rnd)
		if rnd[0] == 0xEF || reservedStart(rnd) || (rnd[4]|rnd[5]|rnd[6]|rnd[7]) == 0 {
			continue
		}
		enc := make([]byte, hsLen)
		newCTR(rnd[skipLen:skipLen+prekeyLen], rnd[skipLen+prekeyLen:skipLen+prekeyLen+ivLen]).XORKeyStream(enc, rnd)
		tail := make([]byte, 8)
		copy(tail, tag[:])
		binary.LittleEndian.PutUint16(tail[4:], uint16(int16(dcIdx)))
		_, _ = rand.Read(tail[6:8])
		out := append([]byte(nil), rnd...)
		for i := 0; i < 8; i++ {
			out[protoTagPos+i] = tail[i] ^ (enc[protoTagPos+i] ^ rnd[protoTagPos+i])
		}
		return out
	}
}

func buildCtx(prekeyIV, secret, relayInit []byte) *cryptoCtx {
	c := &cryptoCtx{}
	c.cltDec = newCTR(keyFor(prekeyIV[:prekeyLen], secret), prekeyIV[prekeyLen:])
	skip(c.cltDec, hsLen) // 64 байт рукопожатия уже прочитаны
	rev := reversed(prekeyIV)
	c.cltEnc = newCTR(keyFor(rev[:prekeyLen], secret), rev[prekeyLen:])

	c.tgEnc = newCTR(relayInit[skipLen:skipLen+prekeyLen], relayInit[skipLen+prekeyLen:skipLen+prekeyLen+ivLen])
	skip(c.tgEnc, hsLen)
	rrev := reversed(relayInit[skipLen : skipLen+prekeyLen+ivLen])
	c.tgDec = newCTR(rrev[:prekeyLen], rrev[prekeyLen:])
	return c
}

// ---------- разбиение потока на MTProto-пакеты ----------

// msgSplitter режет поток на пакеты транспорта, чтобы каждый ушёл отдельным WS-кадром.
type msgSplitter struct {
	proto    uint32
	cbuf     []byte
	pbuf     []byte
	disabled bool
}

func (s *msgSplitter) split(chunk, plain []byte) [][]byte {
	if len(chunk) == 0 {
		return nil
	}
	if s.disabled {
		return [][]byte{chunk}
	}
	s.cbuf = append(s.cbuf, chunk...)
	s.pbuf = append(s.pbuf, plain...)
	var parts [][]byte
	off, total := 0, len(s.cbuf)
	for off < total {
		n, need := s.nextLen(off, total-off)
		if need { // не хватает данных
			break
		}
		if n <= 0 { // нераспознанный формат — отключаем разбиение
			parts = append(parts, append([]byte(nil), s.cbuf[off:]...))
			off = total
			s.disabled = true
			break
		}
		parts = append(parts, append([]byte(nil), s.cbuf[off:off+n]...))
		off += n
	}
	if off > 0 {
		s.cbuf = append(s.cbuf[:0], s.cbuf[off:]...)
		s.pbuf = append(s.pbuf[:0], s.pbuf[off:]...)
	}
	return parts
}

func (s *msgSplitter) flush() []byte {
	if len(s.cbuf) == 0 {
		return nil
	}
	t := s.cbuf
	s.cbuf, s.pbuf = nil, nil
	return t
}

// nextLen: длина следующего пакета; need=true — нужно ещё данных; n=0 — формат не распознан.
func (s *msgSplitter) nextLen(off, avail int) (n int, need bool) {
	switch s.proto {
	case protoAbridged:
		first := s.pbuf[off]
		var payload, hdr int
		if first == 0x7F || first == 0xFF {
			if avail < 4 {
				return 0, true
			}
			payload = int(uint32(s.pbuf[off+1])|uint32(s.pbuf[off+2])<<8|uint32(s.pbuf[off+3])<<16) * 4
			hdr = 4
		} else {
			payload = int(first&0x7F) * 4
			hdr = 1
		}
		if payload <= 0 {
			return 0, false
		}
		if avail < hdr+payload {
			return 0, true
		}
		return hdr + payload, false
	case protoIntermediate, protoPadded:
		if avail < 4 {
			return 0, true
		}
		payload := int(binary.LittleEndian.Uint32(s.pbuf[off:]) & 0x7FFFFFFF)
		if payload <= 0 {
			return 0, false
		}
		if avail < 4+payload {
			return 0, true
		}
		return 4 + payload, false
	}
	return 0, false
}

// ---------- прокси ----------

type TgProxy struct {
	mu      sync.Mutex
	ln      net.Listener
	conns   map[net.Conn]struct{}
	secret  []byte
	port    int
	logf    func(string)
	running bool
}

func NewTgProxy(logf func(string)) *TgProxy {
	return &TgProxy{logf: logf, conns: map[net.Conn]struct{}{}}
}

func (p *TgProxy) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *TgProxy) Start(secretHex string, port int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return nil
	}
	secret, err := hex.DecodeString(secretHex)
	if err != nil || len(secret) != 16 {
		return fmt.Errorf("некорректный секрет")
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)))
	if err != nil {
		return err
	}
	p.ln, p.secret, p.port, p.running = ln, secret, port, true
	go p.acceptLoop(ln)
	return nil
}

func (p *TgProxy) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return
	}
	p.running = false
	_ = p.ln.Close()
	for c := range p.conns {
		_ = c.Close()
	}
}

func (p *TgProxy) track(c net.Conn, add bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if add {
		p.conns[c] = struct{}{}
	} else {
		delete(p.conns, c)
	}
}

func (p *TgProxy) acceptLoop(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go p.handle(c)
	}
}

func (p *TgProxy) handle(c net.Conn) {
	p.track(c, true)
	defer func() { p.track(c, false); _ = c.Close() }()
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}

	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	hs := make([]byte, hsLen)
	if _, err := io.ReadFull(c, hs); err != nil {
		return
	}
	_ = c.SetReadDeadline(time.Time{})

	dc, media, tag, prekeyIV, ok := tryHandshake(hs, p.secret)
	if !ok {
		p.logf("Telegram-прокси: неверный секрет или протокол (настройте прокси в Telegram заново)")
		io.Copy(io.Discard, io.LimitReader(c, 64*1024))
		return
	}
	isTest := dc >= 10000
	if isTest {
		dc -= 10000
	}
	proto := protoPadded
	switch tag {
	case tagAbridged:
		proto = protoAbridged
	case tagIntermediate:
		proto = protoIntermediate
	}
	dcIdx := dc
	if media {
		dcIdx = -dc
	}
	relayInit := genRelayInit(tag, dcIdx)
	ctx := buildCtx(prekeyIV, p.secret, relayInit)

	ws := dialWS(dc, media, isTest)
	if ws == nil {
		p.logf(fmt.Sprintf("Telegram-прокси: DC%d недоступен через WebSocket, пробую напрямую", dc))
		p.tcpFallback(c, dc, isTest, relayInit, ctx)
		return
	}
	defer ws.Close()
	if err := ws.WriteMessage(websocket.BinaryMessage, relayInit); err != nil {
		return
	}
	bridgeWS(c, ws, ctx, &msgSplitter{proto: proto})
}

// dialWS перебирает домены и режимы (обычный, затем fronting) и возвращает готовый WS.
func dialWS(dc int, media, test bool) *websocket.Conn {
	ip, ok := dcRedirects[dc]
	if !ok {
		return nil
	}
	path := wsPath
	if test {
		path += "_test"
	}
	for _, domain := range wsDomains(dc, media) {
		for _, fronted := range []bool{false, true} {
			if c := connectWS(ip, domain, path, fronted); c != nil {
				return c
			}
		}
	}
	return nil
}

func connectWS(ip, domain, path string, fronted bool) *websocket.Conn {
	sni, insecure, timeout := domain, wsInsecureAll, 8*time.Second
	if fronted {
		sni, insecure, timeout = frontingSNI, true, 7*time.Second
	}
	d := websocket.Dialer{
		HandshakeTimeout: timeout,
		Subprotocols:     []string{"binary"},
		ReadBufferSize:   64 * 1024,
		WriteBufferSize:  64 * 1024,
		NetDialTLSContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			raw, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", wsDialAddr(ip))
			if err != nil {
				return nil, err
			}
			if tc, ok := raw.(*net.TCPConn); ok {
				_ = tc.SetNoDelay(true)
			}
			t := tls.Client(raw, &tls.Config{ServerName: sni, InsecureSkipVerify: insecure, MinVersion: tls.VersionTLS12})
			if err := t.HandshakeContext(ctx); err != nil {
				raw.Close()
				return nil, err
			}
			return t, nil
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Second)
	defer cancel()
	c, _, err := d.DialContext(ctx, "wss://"+domain+path, http.Header{})
	if err != nil {
		return nil
	}
	return c
}

// bridgeWS: клиент (TCP) <-> Telegram (WS) с перешифровкой в обе стороны.
func bridgeWS(c net.Conn, ws *websocket.Conn, ctx *cryptoCtx, sp *msgSplitter) {
	done := make(chan struct{}, 2)
	go func() { // клиент -> Telegram
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 64*1024)
		for {
			n, err := c.Read(buf)
			if n > 0 {
				plain := make([]byte, n)
				ctx.cltDec.XORKeyStream(plain, buf[:n])
				enc := make([]byte, n)
				ctx.tgEnc.XORKeyStream(enc, plain)
				for _, part := range sp.split(enc, plain) {
					if werr := ws.WriteMessage(websocket.BinaryMessage, part); werr != nil {
						return
					}
				}
			}
			if err != nil {
				if tail := sp.flush(); tail != nil {
					_ = ws.WriteMessage(websocket.BinaryMessage, tail)
				}
				return
			}
		}
	}()
	go func() { // Telegram -> клиент
		defer func() { done <- struct{}{} }()
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			plain := make([]byte, len(data))
			ctx.tgDec.XORKeyStream(plain, data)
			out := make([]byte, len(data))
			ctx.cltEnc.XORKeyStream(out, plain)
			if _, err := c.Write(out); err != nil {
				return
			}
		}
	}()
	<-done
	_ = ws.Close()
	_ = c.Close()
	<-done
}

// tcpFallback: прямое подключение к DC с перешифровкой (если WS недоступен).
func (p *TgProxy) tcpFallback(c net.Conn, dc int, test bool, relayInit []byte, ctx *cryptoCtx) {
	table := dcDefaultIP
	if test {
		table = dcTestIP
	}
	ip, ok := table[dc]
	if !ok {
		return
	}
	r, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "443"), 10*time.Second)
	if err != nil {
		p.logf(fmt.Sprintf("Telegram-прокси: прямое подключение к DC%d не удалось: %v", dc, err))
		return
	}
	p.track(r, true)
	defer func() { p.track(r, false); r.Close() }()
	if _, err := r.Write(relayInit); err != nil {
		return
	}
	done := make(chan struct{}, 2)
	pipe := func(src, dst net.Conn, dec, enc cipher.Stream) {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 64*1024)
		for {
			n, err := src.Read(buf)
			if n > 0 {
				plain := make([]byte, n)
				dec.XORKeyStream(plain, buf[:n])
				enc.XORKeyStream(plain, plain)
				if _, werr := dst.Write(plain); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}
	go pipe(c, r, ctx.cltDec, ctx.tgEnc)
	go pipe(r, c, ctx.tgDec, ctx.cltEnc)
	<-done
	_ = c.Close()
	_ = r.Close()
	<-done
}
