package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Батники с пробелами и скобками в имени должны запускаться (раньше cmd ломал такую команду).
func TestBatCommandWithParentheses(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"general.bat", "general (ALT).bat", "general (FAKE TLS AUTO ALT2).bat"} {
		marker := filepath.Join(dir, "ran-"+name+".txt")
		script := "@echo off\r\necho ok> \"" + marker + "\"\r\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := batCommand(dir, name)
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("%s: батник не выполнился", name)
		}
	}
}
