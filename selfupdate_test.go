package main

import (
	"errors"
	"testing"
	"time"
)

func TestRetryStart(t *testing.T) {
	busy := errors.New("The process cannot access the file because it is being used by another process.")

	// файл освобождается после нескольких попыток — запуск удаётся
	calls := 0
	err := retryStart(func() error {
		calls++
		if calls < 4 {
			return busy
		}
		return nil
	}, 10, time.Millisecond)
	if err != nil || calls != 4 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}

	// файл так и остаётся занятым — после всех попыток возвращается ошибка
	calls = 0
	err = retryStart(func() error { calls++; return busy }, 5, time.Millisecond)
	if err != busy || calls != 5 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}

	// успех с первого раза — без повторов
	calls = 0
	if err := retryStart(func() error { calls++; return nil }, 5, time.Millisecond); err != nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestVersionLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.7.2", "1.8.0", true}, {"1.8.0", "1.8.1", true}, {"1.8.1", "1.8.0", false},
		{"1.8.0", "1.8.0", false}, {"1.9.0", "1.10.0", true}, {"2.0.0", "1.99.99", false},
	}
	for _, c := range cases {
		if got := versionLess(c.a, c.b); got != c.want {
			t.Errorf("versionLess(%s,%s)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}
