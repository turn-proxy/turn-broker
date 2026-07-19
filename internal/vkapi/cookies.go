package vkapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	http "github.com/bogdanfinn/fhttp"
)

type cookieEntry struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain,omitempty"`
}

func CookiesFromJSON(raw []byte) ([]*http.Cookie, error) {
	var entries []cookieEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parse cookies json: %w", err)
	}
	out := make([]*http.Cookie, 0, len(entries))
	for _, c := range entries {
		if !isVKCookieHost(strings.TrimPrefix(c.Domain, ".")) {
			continue
		}
		out = append(out, &http.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no vk cookies found in export")
	}
	return out, nil
}

func isVKCookieHost(host string) bool {
	if host == "" {
		return true
	}
	for _, base := range []string{"vk.ru", "vk.com"} {
		if host == base || strings.HasSuffix(host, "."+base) {
			return true
		}
	}
	return false
}

func WriteCookiesFile(path string, cookies []*http.Cookie) error {
	entries := make([]cookieEntry, 0, len(cookies))
	for _, c := range cookies {
		domain := c.Domain
		if domain == "" {
			domain = "." + vkDomain
		}
		entries = append(entries, cookieEntry{Name: c.Name, Value: c.Value, Domain: domain})
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize cookies: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename %s -> %s: %w", filepath.Base(tmpName), filepath.Base(path), err)
	}
	return nil
}
