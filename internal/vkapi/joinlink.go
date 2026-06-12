package vkapi

import (
	"fmt"
	"net/url"
	"strings"
)

func ExtractCallToken(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse join_link: %w", err)
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("join_link must be https, got %s", u.Scheme)
	}
	if u.Host != "vk.com" && u.Host != "vk.ru" {
		return "", fmt.Errorf("join_link host must be vk.com or vk.ru, got %s", u.Host)
	}
	var segs []string
	for _, s := range strings.Split(u.Path, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	if len(segs) < 3 || segs[0] != "call" || segs[1] != "join" {
		return "", fmt.Errorf("expected /call/join/<token>, got %s", u.Path)
	}
	return segs[2], nil
}
