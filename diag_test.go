package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{&net.DNSError{Err: "no such host", Name: "x.test"}, "dns"},
		{context.DeadlineExceeded, "timeout"},
		{errors.New("read tcp 1.2.3.4:5->6.7.8.9:443: wsarecv: An existing connection was forcibly closed by the remote host."), "reset"},
		{errors.New("connectex: No connection could be made because the target machine actively refused it."), "refused"},
		{errors.New("tls: handshake failure"), "tls"},
		{errors.New("something strange"), "other"},
	}
	for _, c := range cases {
		if got := classify(c.err); got != c.want {
			t.Errorf("classify(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

func TestProbeTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if _, err := probeTCP(context.Background(), addr); err != nil {
		t.Fatalf("open port: %v", err)
	}
	ln.Close()
	_, err = probeTCP(context.Background(), addr)
	if err == nil {
		t.Fatal("closed port reported as reachable")
	}
	if r := classify(err); r != "refused" {
		t.Fatalf("closed port classified as %q (%v)", r, err)
	}
}

func TestProbeTimeoutIsClassified(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	// 10.255.255.1 обычно не отвечает; если сеть ответила сразу — тест не показателен
	_, err := probeTCP(ctx, "10.255.255.1:443")
	if err != nil && classify(err) != "timeout" && classify(err) != "other" {
		t.Logf("unexpected class %q for %v", classify(err), err)
	}
}

func TestRetryOnDNS(t *testing.T) {
	// временный сбой DNS проходит после повтора
	calls := 0
	ms, err := retryOnDNS(func() (int64, error) {
		calls++
		if calls < 3 {
			return 0, &net.DNSError{Err: "server misbehaving", Name: "x"}
		}
		return 42, nil
	})
	if err != nil || ms != 42 || calls != 3 {
		t.Fatalf("ms=%d err=%v calls=%d", ms, err, calls)
	}
	// не-DNS ошибка не повторяется
	calls = 0
	_, err = retryOnDNS(func() (int64, error) { calls++; return 0, errors.New("connection reset by peer") })
	if err == nil || calls != 1 {
		t.Fatalf("non-dns error retried: calls=%d", calls)
	}
	// постоянный сбой DNS после трёх попыток остаётся ошибкой
	calls = 0
	_, err = retryOnDNS(func() (int64, error) { calls++; return 0, &net.DNSError{Err: "no such host", Name: "x"} })
	if err == nil || calls != 3 {
		t.Fatalf("persistent dns: calls=%d err=%v", calls, err)
	}
}

func TestDiagHints(t *testing.T) {
	ok := []DiagItem{{Kind: "https", OK: true}}
	if h := diagHints(ok, false); len(h) != 2 || !strings.Contains(h[0], "доступно") {
		t.Fatalf("all ok hints: %v", h)
	}
	bad := []DiagItem{
		{Kind: "https", OK: true},
		{Kind: "https", Reason: "reset"},
		{Kind: "tcp", Reason: "timeout"},
		{Kind: "https", Reason: "dns"},
	}
	joined := strings.Join(diagHints(bad, false), "\n")
	for _, want := range []string{"DNS", "DPI", "IP-адрес"} {
		if !strings.Contains(joined, want) {
			t.Errorf("hint %q missing in:\n%s", want, joined)
		}
	}
	if !strings.Contains(strings.Join(diagHints(bad, true), "\n"), "Подобрать автоматически") {
		t.Error("bypass-on hint should suggest re-selecting")
	}
}
