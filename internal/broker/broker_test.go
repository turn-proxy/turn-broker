package broker

import (
	"slices"
	"testing"
	"time"

	"github.com/turn-proxy/turn-broker/internal/turn"
	"github.com/turn-proxy/turn-broker/proto"
)

func TestBuildSession(t *testing.T) {
	creds := turn.Creds{URLs: []string{"a", "b"}, Username: "u", Credential: "c", ExpiresAt: 100}

	s := buildSession(creds, "link", 12345)
	if s.RefreshedAt != 12345 {
		t.Fatalf("RefreshedAt = %d, want 12345", s.RefreshedAt)
	}
	if s.JoinLink != "link" || s.Username != "u" || s.Credential != "c" || s.ExpiresAt != 100 {
		t.Fatalf("unexpected session fields: %+v", s)
	}
	if !slices.Equal(s.URLs, creds.URLs) {
		t.Fatalf("URLs = %v, want %v", s.URLs, creds.URLs)
	}
}

func TestEvictIfIdle(t *testing.T) {
	c := NewSessionCache()
	c.replaceCreds(proto.Session{JoinLink: "link"})

	if c.evictIfIdle("link", time.Hour) {
		t.Fatal("fresh session should not be evicted")
	}
	if _, ok := c.Session("link"); !ok {
		t.Fatal("session should still be present")
	}

	c.mu.Lock()
	c.sessions["link"].lastSeen = time.Now().Add(-time.Hour).Unix()
	c.mu.Unlock()

	if !c.evictIfIdle("link", time.Minute) {
		t.Fatal("idle session should be evicted")
	}
	if _, ok := c.Session("link"); ok {
		t.Fatal("session should be removed after eviction")
	}

	if !c.evictIfIdle("missing", time.Minute) {
		t.Fatal("missing key should report evicted/gone")
	}
}

func TestTouchUpdatesLastSeen(t *testing.T) {
	c := NewSessionCache()
	c.replaceCreds(proto.Session{JoinLink: "link"})

	c.mu.Lock()
	c.sessions["link"].lastSeen = time.Now().Add(-time.Hour).Unix()
	old := c.sessions["link"].lastSeen
	c.mu.Unlock()

	c.touch("link")

	c.mu.RLock()
	got := c.sessions["link"].lastSeen
	c.mu.RUnlock()
	if got <= old {
		t.Fatalf("touch did not advance lastSeen: got %d, old %d", got, old)
	}
}
