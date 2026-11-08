package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Клиент Telegram по спецификации obfuscated2 — написан независимо от кода прокси.
func fakeClientInit(t *testing.T, secret []byte, tag [4]byte, dcIdx int) (init []byte, enc, dec cipherStream) {
	p := make([]byte, 64)
	rand.Read(p)
	copy(p[56:60], tag[:])
	binary.LittleEndian.PutUint16(p[60:], uint16(int16(dcIdx)))

	k := sha256.Sum256(append(append([]byte{}, p[8:40]...), secret...))
	e := newCTR(k[:], p[40:56])
	c := make([]byte, 64)
	e.XORKeyStream(c, p)
	init = append(append([]byte{}, p[:56]...), c[56:64]...)

	rev := reversed(p[8:56])
	k2 := sha256.Sum256(append(append([]byte{}, rev[:32]...), secret...))
	return init, e, newCTR(k2[:], rev[32:48])
}

type cipherStream interface{ XORKeyStream(dst, src []byte) }

func TestHandshakeAndRelayInit(t *testing.T) {
	secret := make([]byte, 16)
	rand.Read(secret)
	init, _, _ := fakeClientInit(t, secret, tagIntermediate, -4)
	dc, media, tag, _, ok := tryHandshake(init, secret)
	if !ok || dc != 4 || !media || tag != tagIntermediate {
		t.Fatalf("handshake: dc=%d media=%v tag=%x ok=%v", dc, media, tag, ok)
	}
	// неверный секрет не должен проходить
	bad := make([]byte, 16)
	rand.Read(bad)
	if _, _, _, _, ok := tryHandshake(init, bad); ok {
		t.Fatal("handshake accepted with wrong secret")
	}
	// сервер Telegram разбирает relay init без секрета
	rel := genRelayInit(tagAbridged, 2)
	plain := make([]byte, 64)
	newCTR(rel[8:40], rel[40:56]).XORKeyStream(plain, rel)
	if !bytes.Equal(plain[56:60], tagAbridged[:]) || int16(binary.LittleEndian.Uint16(plain[60:])) != 2 {
		t.Fatalf("relay init decodes to tag=%x dc=%d", plain[56:60], int16(binary.LittleEndian.Uint16(plain[60:])))
	}
}

func TestEndToEndThroughFakeWS(t *testing.T) {
	var mu sync.Mutex
	var messages [][]byte
	var gotDC int

	up := websocket.Upgrader{Subprotocols: []string{"binary"}, CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Host, "kws2.web.telegram.org") || r.URL.Path != "/apiws" {
			http.Error(w, "bad", 400)
			return
		}
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		_, init, err := c.ReadMessage()
		if err != nil || len(init) != 64 {
			return
		}
		sdec := newCTR(init[8:40], init[40:56])
		plain := make([]byte, 64)
		sdec.XORKeyStream(plain, init)
		mu.Lock()
		gotDC = int(int16(binary.LittleEndian.Uint16(plain[60:])))
		mu.Unlock()
		rev := reversed(init[8:56])
		senc := newCTR(rev[:32], rev[32:48])
		for {
			_, m, err := c.ReadMessage()
			if err != nil {
				return
			}
			mu.Lock()
			messages = append(messages, m)
			mu.Unlock()
			pl := make([]byte, len(m))
			sdec.XORKeyStream(pl, m)
			out := make([]byte, len(m))
			senc.XORKeyStream(out, pl)
			c.WriteMessage(websocket.BinaryMessage, out)
		}
	}))
	defer srv.Close()

	oldAddr, oldIns := wsDialAddr, wsInsecureAll
	wsDialAddr = func(string) string { return srv.Listener.Addr().String() }
	wsInsecureAll = true
	defer func() { wsDialAddr, wsInsecureAll = oldAddr, oldIns }()

	secret := make([]byte, 16)
	rand.Read(secret)
	p := NewTgProxy(func(s string) { t.Log(s) })
	if err := p.Start(hex.EncodeToString(secret), 0); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	conn, err := net.Dial("tcp", p.ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	init, cenc, cdec := fakeClientInit(t, secret, tagAbridged, 2)
	conn.Write(init)

	// два абридженных пакета в одной записи: прокси обязан отправить их отдельными WS-сообщениями
	mk := func(n int) []byte {
		b := make([]byte, 1+n)
		b[0] = byte(n / 4)
		rand.Read(b[1:])
		return b
	}
	p1, p2 := mk(16), mk(32)
	plain := append(append([]byte{}, p1...), p2...)
	enc := make([]byte, len(plain))
	cenc.XORKeyStream(enc, plain)
	conn.Write(enc)

	got := make([]byte, 0, len(plain))
	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for len(got) < len(plain) {
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("read: %v (got %d of %d)", err, len(got), len(plain))
		}
		got = append(got, buf[:n]...)
	}
	dec := make([]byte, len(got))
	cdec.XORKeyStream(dec, got)
	if !bytes.Equal(dec, plain) {
		t.Fatalf("echo mismatch:\n got %x\nwant %x", dec, plain)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotDC != 2 {
		t.Fatalf("server saw dc=%d", gotDC)
	}
	if len(messages) != 2 || len(messages[0]) != len(p1) || len(messages[1]) != len(p2) {
		t.Fatalf("splitter: got %d messages", len(messages))
	}
}
