package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeDomains(t *testing.T) {
	good, bad := normalizeDomains("https://www.Example.com/path?x=1, game.net\nfoo\n# c\nexample.com")
	if strings.Join(good, "|") != "example.com|game.net" {
		t.Fatalf("good=%v", good)
	}
	if len(bad) != 1 || bad[0] != "foo" {
		t.Fatalf("bad=%v", bad)
	}
}

func TestApplyDomainsKeepsUserContent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("ProgramData", tmp)
	lists := filepath.Join(zapretDir(), "lists")
	os.MkdirAll(lists, 0o755)
	p := filepath.Join(lists, "list-general-user.txt")
	os.WriteFile(p, []byte("mine.org\r\n"), 0o644)

	applyDomains("a.com\nb.com")
	applyDomains("c.com") // повторное применение заменяет блок, а не дублирует
	b, _ := os.ReadFile(p)
	s := string(b)
	if !strings.Contains(s, "mine.org") || !strings.Contains(s, "c.com") || strings.Contains(s, "a.com") {
		t.Fatalf("unexpected: %q", s)
	}
	if strings.Count(s, blockStart) != 1 {
		t.Fatalf("block duplicated: %q", s)
	}
	// файл не должен накапливать пустые строки и «\r\r\n» при повторных применениях
	for i := 0; i < 5; i++ {
		applyDomains("c.com")
	}
	b, _ = os.ReadFile(p)
	if strings.Contains(string(b), "\r\r") || strings.Contains(string(b), "\r\n\r\n") {
		t.Fatalf("blank lines accumulated: %q", string(b))
	}
}
