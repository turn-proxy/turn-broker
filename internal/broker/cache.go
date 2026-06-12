package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/turn-proxy/turn-broker/internal/turn"
	"github.com/turn-proxy/turn-broker/proto"
	"golang.org/x/sync/singleflight"
)

type entry struct {
	session  proto.Session
	lastSeen int64
}

type SessionCache struct {
	mu        sync.RWMutex
	sessions  map[string]*entry
	initGroup singleflight.Group
}

func NewSessionCache() *SessionCache {
	return &SessionCache{sessions: map[string]*entry{}}
}

func (c *SessionCache) Session(joinLink string) (proto.Session, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.sessions[joinLink]
	if !ok {
		return proto.Session{}, false
	}
	return e.session, true
}

func (c *SessionCache) touch(joinLink string) {
	c.mu.Lock()
	if e, ok := c.sessions[joinLink]; ok {
		e.lastSeen = time.Now().Unix()
	}
	c.mu.Unlock()
}

func (c *SessionCache) replaceCreds(next proto.Session) {
	c.mu.Lock()
	if e, ok := c.sessions[next.JoinLink]; ok {
		e.session = next
	} else {
		c.sessions[next.JoinLink] = &entry{session: next, lastSeen: time.Now().Unix()}
	}
	c.mu.Unlock()
}

func (c *SessionCache) remove(joinLink string) {
	c.mu.Lock()
	delete(c.sessions, joinLink)
	c.mu.Unlock()
}

func (c *SessionCache) evictIfIdle(joinLink string, ttl time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.sessions[joinLink]
	if !ok {
		return true
	}
	if time.Now().Unix()-e.lastSeen < int64(ttl.Seconds()) {
		return false
	}
	delete(c.sessions, joinLink)
	return true
}

func InitializeSession(reqCtx, rootCtx context.Context, cache *SessionCache, prov turn.Provider, joinLink string, refreshInterval time.Duration) (proto.Session, error) {
	if existing, ok := cache.Session(joinLink); ok {
		cache.touch(joinLink)
		return existing, nil
	}

	res, err, _ := cache.initGroup.Do(joinLink, func() (any, error) {
		if existing, ok := cache.Session(joinLink); ok {
			cache.touch(joinLink)
			return existing, nil
		}
		creds, err := prov.Fetch(reqCtx, joinLink)
		if err != nil {
			return proto.Session{}, fmt.Errorf("initial fetch: %w", err)
		}
		session := buildSession(creds, joinLink, uint64(time.Now().Unix()))
		slog.Info("session initialized",
			"join_link", joinLink,
			"urls", strings.Join(session.URLs, ", "),
			"refreshed_at", session.RefreshedAt,
			"expires_at", session.ExpiresAt)
		cache.replaceCreds(session)

		go refreshLoop(rootCtx, cache, prov, joinLink, refreshInterval)
		return session, nil
	})
	if err != nil {
		return proto.Session{}, err
	}
	return res.(proto.Session), nil
}

func buildSession(creds turn.Creds, joinLink string, refreshedAt uint64) proto.Session {
	return proto.Session{
		RefreshedAt: refreshedAt,
		Username:    creds.Username,
		Credential:  creds.Credential,
		URLs:        creds.URLs,
		JoinLink:    joinLink,
		ExpiresAt:   creds.ExpiresAt,
	}
}

const (
	minRefreshDelay   = time.Second
	refreshErrBackoff = 30 * time.Second
	sessionIdleTTL    = time.Hour
)

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func nextRefreshDelay(cache *SessionCache, joinLink string, refreshInterval time.Duration) time.Duration {
	var delay time.Duration
	if s, ok := cache.Session(joinLink); ok {
		now := uint64(time.Now().Unix())
		if s.ExpiresAt > now {
			delay = time.Duration(s.ExpiresAt-now) * time.Second
		}
		if refreshInterval > 0 && refreshInterval < delay {
			delay = refreshInterval
		}
	}
	if delay < minRefreshDelay {
		delay = minRefreshDelay
	}
	return delay
}

func refreshLoop(ctx context.Context, cache *SessionCache, prov turn.Provider, joinLink string, refreshInterval time.Duration) {
	for {
		delay := nextRefreshDelay(cache, joinLink, refreshInterval)
		wake := delay
		if sessionIdleTTL < wake {
			wake = sessionIdleTTL
		}
		if wake == delay {
			slog.Info("scheduled next refresh", "join_link", joinLink, "next_refresh_s", int64(delay.Seconds()))
		}
		if !sleepCtx(ctx, wake) {
			return
		}
		if cache.evictIfIdle(joinLink, sessionIdleTTL) {
			slog.Info("session idle, evicted", "join_link", joinLink)
			return
		}
		if wake < delay {
			continue
		}

		creds, err := prov.Fetch(ctx, joinLink)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if errors.Is(err, turn.ErrAuthExpired) || errors.Is(err, turn.ErrCallGone) {
				slog.Warn("terminal fetch error, dropping session", "join_link", joinLink, "err", err)
				cache.remove(joinLink)
				return
			}
			if s, ok := cache.Session(joinLink); ok && uint64(time.Now().Unix()) >= s.ExpiresAt {
				slog.Warn("refresh failing and creds expired, dropping session", "join_link", joinLink, "err", err)
				cache.remove(joinLink)
				return
			}
			slog.Warn("refresh failed, keeping stale session", "join_link", joinLink, "err", err, "retry_in_s", int64(refreshErrBackoff.Seconds()))
			if !sleepCtx(ctx, refreshErrBackoff) {
				return
			}
			continue
		}
		var prev *proto.Session
		if p, ok := cache.Session(joinLink); ok {
			prev = &p
		}
		session := buildSession(creds, joinLink, uint64(time.Now().Unix()))
		if prev == nil || !slices.Equal(prev.URLs, session.URLs) {
			slog.Info("refreshed session (cluster change)",
				"join_link", joinLink, "urls", strings.Join(session.URLs, ", "),
				"refreshed_at", session.RefreshedAt, "expires_at", session.ExpiresAt)
		} else {
			slog.Info("refreshed session", "join_link", joinLink,
				"refreshed_at", session.RefreshedAt, "expires_at", session.ExpiresAt)
		}
		cache.replaceCreds(session)
	}
}
