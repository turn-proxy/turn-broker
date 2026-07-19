# turn-broker

HTTP service that uses VK cookies to fetch TURN creds and serves them to
turn-proxy clients.

## Layout

Module `github.com/turn-proxy/turn-broker`, one binary (`cmd/turn-broker`):

| Path | Purpose |
|---|---|
| `cmd/turn-broker` | Entrypoint: load config, build `vkprovider`, run broker |
| `proto` | Public wire type `Session` (creds + URLs + `refreshed_at`), served to clients |
| `internal/turn` | Provider-agnostic contract: `Creds`, `Provider` interface, `ErrAuthExpired` sentinel |
| `internal/broker` | `SessionCache` + lazy per-link init + refresh loop + HTTP API. Talks only to `turn.Provider` (no VK imports) |
| `internal/vkprovider` | VK implementation of `turn.Provider`: link parsing + 5-step flow + cookie persistence + auth-failure mapping |
| `internal/vkapi` | Cookie-authed VK client: web_token → calls.getSettings → messages.getCallToken → auth.anonymLogin → vchat.joinConversationByLink. HTTPS via `bogdanfinn/tls-client` (`transport.go`) |

## HTTP API

Creds only — no peer rendezvous (the client learns the server address from its own
`upstream`). **GET-only** to survive a CDN that blocks POST.

- `GET /v1/session?join_link=…` — lazily inits the session on first hit (spawns a per-link refresh goroutine), returns the cached `Session`. Idempotent; client polls on a 60s timer. Any number of `join_link`s coexist
- `GET /healthz`

Refresh sleeps until `expires_at` (from the TURN username's `<unix>:` prefix),
capped by optional `vk.session_refresh_secs`; 30s backoff on transient errors while
the cached creds are still valid. The session is dropped (loop exits) on terminal
errors — provider auth expiry (`turn.ErrAuthExpired`, e.g. VK API error 5 / expired
cookies) or an unjoinable call (`turn.ErrCallGone`, a non-transient VK error from the
join). The provider maps VK codes: `vkapi.IsTransient` (1/6/9/10) → keep stale + retry;
anything else → terminal. Backstops: a refresh that keeps failing past the creds'
`expires_at` drops the session, as does no client request for `sessionIdleTTL` (1h).
An initial fetch failure returns 500 and creates no session. After a drop, the next
request re-inits it. `Session.refreshed_at` is the unix time of the last successful
fetch. After each successful fetch the (rotated) cookies are written back to
`vk.cookies_file` — one file shared across all sessions.

## VK API client

`vkapi` authenticates with browser cookies (JSON `{name, value, domain}` array;
`CookiesFromJSON` keeps vk.ru/vk.com entries — VK migrated to the `vk.ru` domain
zone, endpoints are `api.vk.ru`/`login.vk.ru`, cookies land on `*.vk.ru`, `.com`
still accepted for back-compat via `isVKCookieHost`). HTTPS via `bogdanfinn/tls-client`
(`transport.go`, profile `Chrome_146`) matches a real Chrome fingerprint at both
the TLS (JA3/JA4) and HTTP/2 (akamai SETTINGS, window update, pseudo-header order)
layers. The tls-client cookie jar seeds from the file, then attaches/absorbs per
request — VK's `Set-Cookie` rotation is captured and scoped per registrable domain
(vk.ru → `*.vk.ru`, nothing leaks to the dynamic `calls.okcdn.ru` host).
`SnapshotCookies` dumps the vk.ru/vk.com buckets with domains preserved for write-back.
`WebToken` is cached, refreshed 300s before expiry. Creds expiry from the TURN
username prefix (`parseExpiryFromUsername`).

## Build / run

```bash
make build      # → dist/turn-broker
make test
make vet

./dist/turn-broker -config turn-broker.json
```

Config path also via `TURN_BROKER_CONFIG`. The live TLS+h2 fingerprint check
(`tls.peet.ws` etc.) is behind the `netcheck` build tag.

## Config

`turn-broker.json`:
```json
{
  "bind": "0.0.0.0:8787",
  "vk": {
    "cookies_file": "/path/to/vk-cookies.json",
    "session_refresh_secs": 300
  }
}
```

- `vk.session_refresh_secs` (optional): min refresh cadence, capped to remaining creds lifetime. Unset = refresh at expiry (~8h)
- `vk.app_id`, `vk.api_version` (optional): override defaults (`6287487`, `5.275`)

## Code conventions

- **NEVER write comments** (not even doc comments) unless explicitly asked.
- Commit messages: terse, single-line, imperative, no trailing period.

## Useful files

- `proto/session.go` — `Session` wire DTO
- `internal/turn/turn.go` — `Provider` interface, `Creds`, `ErrAuthExpired`
- `internal/broker/cache.go` — `SessionCache`, lazy init, refresh loop (provider-agnostic)
- `internal/broker/api.go` — `/v1/session` + `/healthz`
- `internal/vkprovider/vkprovider.go` — `turn.Provider` impl: VK 5-step flow + cookie persistence
- `internal/vkapi/client.go` — the 5-step call-token / TURN-creds flow
- `internal/vkapi/transport.go` — `tls-client` Chrome-fingerprint HTTP client