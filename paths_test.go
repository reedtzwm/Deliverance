package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataDirMigratesLegacyFolder(t *testing.T) {
	base := t.TempDir()
	t.Setenv("ProgramData", base)
	old := filepath.Join(base, "ZapretGUI")
	os.MkdirAll(filepath.Join(old, "zapret"), 0o755)
	os.WriteFile(filepath.Join(old, "config.json"), []byte(`{"selectedBat":"general.bat"}`), 0o644)

	d := dataDir()
	if filepath.Base(d) != "Deliverance" {
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
	// повторный вызов ничего не ломает
	if dataDir() != d {
		t.Fatal("dataDir not stable")
	}
}
