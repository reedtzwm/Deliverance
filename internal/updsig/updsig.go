// Package updsig подписывает и проверяет файлы обновлений приложения.
//
// Схема: Ed25519-подпись от SHA-256 файла с доменным префиксом.
// Приватный ключ хранится только у автора, публичный вшит в программу.
package updsig

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
)

const domain = "zapretgui-update-v1\n"

func message(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return append([]byte(domain), h.Sum(nil)...), nil
}

// GenerateKey возвращает пару ключей в hex: публичный (32 байта) и приватный (64 байта).
func GenerateKey() (pubHex, privHex string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return hex.EncodeToString(pub), hex.EncodeToString(priv), nil
}

// SignFile возвращает подпись файла в hex.
func SignFile(privHex, path string) (string, error) {
	b, err := hex.DecodeString(strings.TrimSpace(privHex))
	if err != nil || len(b) != ed25519.PrivateKeySize {
		return "", errors.New("некорректный приватный ключ")
	}
	msg, err := message(path)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(ed25519.Sign(ed25519.PrivateKey(b), msg)), nil
}

// VerifyFile проверяет подпись файла публичным ключом.
func VerifyFile(pubHex, sigHex, path string) error {
	pub, err := hex.DecodeString(strings.TrimSpace(pubHex))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return errors.New("в программе не задан публичный ключ обновлений")
	}
	sig, err := hex.DecodeString(strings.TrimSpace(sigHex))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return errors.New("файл подписи повреждён")
	}
	msg, err := message(path)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), msg, sig) {
		return errors.New("подпись не совпадает: файл изменён или подписан другим ключом")
	}
	return nil
}
