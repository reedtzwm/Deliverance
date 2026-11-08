package updsig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSignVerify(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "setup.exe")
	os.WriteFile(f, []byte("installer-bytes"), 0o644)

	sig, err := SignFile(priv, f)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyFile(pub, sig, f); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}

	// изменённый файл
	os.WriteFile(f, []byte("installer-bytes!"), 0o644)
	if VerifyFile(pub, sig, f) == nil {
		t.Fatal("tampered file accepted")
	}

	// чужой ключ
	os.WriteFile(f, []byte("installer-bytes"), 0o644)
	pub2, _, _ := GenerateKey()
	if VerifyFile(pub2, sig, f) == nil {
		t.Fatal("signature accepted under another key")
	}

	// пустой публичный ключ в программе — обновления отклоняются
	if VerifyFile("", sig, f) == nil {
		t.Fatal("empty public key accepted")
	}
}
