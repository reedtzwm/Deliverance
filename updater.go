package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const repo = "Flowseal/zapret-discord-youtube"

func installedVersion() string {
	b, err := os.ReadFile(versionPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// latestTag узнаёт последний релиз через редирект /releases/latest —
// это не расходует лимит GitHub API (60 запросов/час на IP).
func latestTag(ctx context.Context, repo string) (string, error) {
	client := &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, "https://github.com/"+repo+"/releases/latest", nil)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("GitHub не вернул редирект (HTTP %d)", resp.StatusCode)
	}
	i := strings.LastIndex(loc, "/")
	return loc[i+1:], nil
}

// installRelease скачивает и распаковывает релиз в zapretDir,
// сохраняя пользовательские списки (*-user.txt).
func installRelease(ctx context.Context, tag string, logf func(string)) error {
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/zapret-discord-youtube-%s.zip", repo, tag, tag)
	logf("Скачиваю " + url)

	tmp, err := os.CreateTemp("", "zapret-*.zip")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		tmp.Close()
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		tmp.Close()
		return fmt.Errorf("загрузка: HTTP %d", resp.StatusCode)
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	// распаковка во временную папку рядом, затем подмена
	newDir := zapretDir() + ".new"
	_ = os.RemoveAll(newDir)
	if err := unzipStrip(tmpName, newDir); err != nil {
		return err
	}
	// отключаем встроенную проверку обновлений батников — этим занимается приложение
	_ = os.Remove(filepath.Join(newDir, "utils", "check_updates.enabled"))

	// переносим пользовательские списки
	old := zapretDir()
	if matches, _ := filepath.Glob(filepath.Join(old, "lists", "*-user.txt")); len(matches) > 0 {
		for _, m := range matches {
			if b, err := os.ReadFile(m); err == nil {
				_ = os.WriteFile(filepath.Join(newDir, "lists", filepath.Base(m)), b, 0o644)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(newDir, ".version"), []byte(tag), 0o644); err != nil {
		return err
	}

	_ = os.RemoveAll(old)
	if err := os.Rename(newDir, old); err != nil {
		return err
	}
	logf("Установлена версия " + tag)
	return nil
}

// unzipStrip распаковывает zip, отбрасывая единственную верхнюю папку.
func unzipStrip(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	destAbs, _ := filepath.Abs(dest)
	for _, f := range r.File {
		name := f.Name
		if i := strings.Index(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		if name == "" {
			continue
		}
		target := filepath.Join(destAbs, filepath.FromSlash(name))
		if !strings.HasPrefix(target, destAbs+string(os.PathSeparator)) {
			return fmt.Errorf("небезопасный путь в архиве: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0o755)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(target), 0o755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
