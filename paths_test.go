package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Данные прежних названий (Deliverance, ZapretGUI) должны переезжать в папку DZapret.
func TestDataDirMigratesLegacyFolders(t *testing.T) {
	for _, legacy := range []string{"Deliverance", "ZapretGUI"} {
		t.Run(legacy, func(t *testing.T) {
			base := t.TempDir()
			t.Setenv("ProgramData", base)
			old := filepath.Join(base, legacy)
			os.MkdirAll(filepath.Join(old, "zapret"), 0o755)
			os.WriteFile(filepath.Join(old, "config.json"), []byte(`{"selectedBat":"general.bat"}`), 0o644)

			d := dataDir()
			if filepath.Base(d) != "DZapret" {
				t.Fatalf("dataDir = %s", d)
			}
			if _, err := os.Stat(filepath.Join(d, "config.json")); err != nil {
				t.Fatalf("config not migrated: %v", err)
			}
			if _, err := os.Stat(old); err == nil {
				t.Fatal("old folder still exists")
			}
			if loadConfig().SelectedBat != "general.bat" {
				t.Fatal("selected bat lost after migration")
			}
			if dataDir() != d { // повторный вызов ничего не ломает
				t.Fatal("dataDir not stable")
			}
		})
	}
}

// Метки блока со старыми названиями заменяются, а не копятся.
func TestLegacyDomainBlockReplaced(t *testing.T) {
	t.Setenv("ProgramData", t.TempDir())
	lists := filepath.Join(zapretDir(), "lists")
	os.MkdirAll(lists, 0o755)
	p := filepath.Join(lists, "list-general-user.txt")
	old := "mine.org\n" + legacyBlocks[0][0] + "\nold.com\n" + legacyBlocks[0][1] + "\n"
	os.WriteFile(p, []byte(old), 0o644)

	applyDomains("new.com")
	b, _ := os.ReadFile(p)
	s := string(b)
	if !strings.Contains(s, "mine.org") || !strings.Contains(s, "new.com") ||
		strings.Contains(s, "old.com") || strings.Contains(s, "Deliverance") {
		t.Fatalf("legacy block not replaced: %q", s)
	}
}
