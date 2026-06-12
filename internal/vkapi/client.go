package vkapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
)

const (
	DefaultAppID      = "6287487"
	DefaultAPIVersion = "5.275"

	vkAPIBase   = "https://api.vk.com"
	vkLoginBase = "https://login.vk.com"

	webTokenRefreshBufferSecs = 300
)

type Client struct {
	http       tlsclient.HttpClient
	appID      string
	apiVersion string

	jar tlsclient.CookieJar

	mu      sync.Mutex
	session *webTokenData
}

type Builder struct {
	cookies    []*http.Cookie
	appID      string
	apiVersion string
	http       tlsclient.HttpClient
}

func NewBuilder(cookies []*http.Cookie) *Builder {
	return &Builder{
		cookies:    cookies,
		appID:      DefaultAppID,
		apiVersion: DefaultAPIVersion,
	}
}

func (b *Builder) AppID(v string) *Builder {
	if v != "" {
		b.appID = v
	}
	return b
}

func (b *Builder) APIVersion(v string) *Builder {
	if v != "" {
		b.apiVersion = v
	}
	return b
}

func (b *Builder) HTTPClient(c tlsclient.HttpClient) *Builder {
	b.http = c
	return b
}

func (b *Builder) Build() (*Client, error) {
	if len(b.cookies) == 0 {
		return nil, fmt.Errorf("vkapi: no cookies provided")
	}
	jar := tlsclient.NewCookieJar()
	seedJar(jar, b.cookies)
	client := b.http
	if client == nil {
		c, err := newChromeClient(jar)
		if err != nil {
			return nil, fmt.Errorf("vkapi: build http client: %w", err)
		}
		client = c
	}
	return &Client{
		http:       client,
		appID:      b.appID,
		apiVersion: b.apiVersion,
		jar:        jar,
	}, nil
}

func seedJar(jar tlsclient.CookieJar, cookies []*http.Cookie) {
	for _, c := range cookies {
		host := strings.TrimPrefix(c.Domain, ".")
		if host == "" {
			host = "vk.com"
		}
		jar.SetCookies(&url.URL{Scheme: "https", Host: host}, []*http.Cookie{c})
	}
}

func (v *Client) AppID() string      { return v.appID }
func (v *Client) APIVersion() string { return v.apiVersion }

func (v *Client) SnapshotCookies() []*http.Cookie {
	if v.jar == nil {
		return nil
	}
	var out []*http.Cookie
	for host, bucket := range v.jar.GetAllCookies() {
		if host != "vk.com" {
			continue
		}
		out = append(out, bucket...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (v *Client) send(ctx context.Context, endpoint string, query, form url.Values) ([]byte, error) {
	if query != nil {
		endpoint = endpoint + "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://vk.com")
	req.Header.Set("Referer", "https://vk.com/")

	resp, err := v.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http send: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if ae, ok := apiErrorFromBody(body); ok {
		return nil, ae
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, body)
	}
	return body, nil
}

func (v *Client) WebToken(ctx context.Context) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if s := v.session; s != nil && (s.Expires == 0 || uint64(time.Now().Unix())+webTokenRefreshBufferSecs < s.Expires) {
		return s.AccessToken, nil
	}
	fresh, err := v.fetchWebToken(ctx)
	if err != nil {
		return "", err
	}
	v.session = fresh
	return fresh.AccessToken, nil
}

func (v *Client) fetchWebToken(ctx context.Context) (*webTokenData, error) {
	form := url.Values{"version": {"1"}, "app_id": {v.appID}}
	body, err := v.send(ctx, vkLoginBase+"/?act=web_token", nil, form)
	if err != nil {
		return nil, fmt.Errorf("web_token: %w", err)
	}
	var r webTokenResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("web_token parse: %w", err)
	}
	if r.Data == nil || r.Data.AccessToken == "" {
		return nil, fmt.Errorf("web_token error: type=%s info=%v", r.Type, r.ErrorInfo)
	}
	return r.Data, nil
}

func (v *Client) CallsGetSettings(ctx context.Context) (*CallsSettings, error) {
	token, err := v.WebToken(ctx)
	if err != nil {
		return nil, err
	}
	query := url.Values{"v": {v.apiVersion}, "client_id": {v.appID}}
	form := url.Values{"access_token": {token}}
	body, err := v.send(ctx, vkAPIBase+"/method/calls.getSettings", query, form)
	if err != nil {
		return nil, fmt.Errorf("calls.getSettings: %w", err)
	}
	res, err := decodeResponse[callsGetSettingsResult]("calls.getSettings", body)
	if err != nil {
		return nil, err
	}
	if res.Settings.PublicKey == "" {
		return nil, fmt.Errorf("calls.getSettings: empty public_key")
	}
	return &res.Settings, nil
}

func (v *Client) MessagesGetCallToken(ctx context.Context) (*MessagesCallToken, error) {
	token, err := v.WebToken(ctx)
	if err != nil {
		return nil, err
	}
	query := url.Values{"v": {v.apiVersion}, "client_id": {v.appID}}
	form := url.Values{"access_token": {token}, "env": {"production"}}
	body, err := v.send(ctx, vkAPIBase+"/method/messages.getCallToken", query, form)
	if err != nil {
		return nil, fmt.Errorf("messages.getCallToken: %w", err)
	}
	res, err := decodeResponse[MessagesCallToken]("messages.getCallToken", body)
	if err != nil {
		return nil, err
	}
	if res.Token == "" || res.APIBaseURL == "" {
		return nil, fmt.Errorf("messages.getCallToken: missing token/api_base_url")
	}
	res.APIBaseURL = normalizeFbDo(res.APIBaseURL)
	return res, nil
}

func (v *Client) AuthAnonymLogin(ctx context.Context, apiBaseURL, applicationKey, callToken string) (string, error) {
	deviceID := "vk-api-" + randomDeviceSuffix()
	sessionData, err := json.Marshal(map[string]any{
		"version":        3,
		"device_id":      deviceID,
		"client_version": "1.1",
		"client_type":    "SDK_JS",
		"auth_token":     callToken,
	})
	if err != nil {
		return "", fmt.Errorf("auth.anonymLogin: marshal session_data: %w", err)
	}
	form := url.Values{
		"method":          {"auth.anonymLogin"},
		"application_key": {applicationKey},
		"format":          {"json"},
		"session_data":    {string(sessionData)},
	}
	body, err := v.send(ctx, apiBaseURL, nil, form)
	if err != nil {
		return "", fmt.Errorf("auth.anonymLogin: %w", err)
	}
	var r anonymLoginResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("auth.anonymLogin parse: %w", err)
	}
	if r.SessionKey == "" {
		return "", fmt.Errorf("auth.anonymLogin: no session_key: %s", body)
	}
	return r.SessionKey, nil
}

func (v *Client) VChatJoinConversationByLink(ctx context.Context, apiBaseURL, applicationKey, sessionKey, callToken string) (*VChatJoinResponse, error) {
	mediaSettings := `{"isAudioEnabled":false,"isVideoEnabled":true,"isScreenSharingEnabled":false}`
	form := url.Values{
		"method":          {"vchat.joinConversationByLink"},
		"session_key":     {sessionKey},
		"application_key": {applicationKey},
		"joinLink":        {callToken},
		"isVideo":         {"true"},
		"isAudio":         {"false"},
		"mediaSettings":   {mediaSettings},
		"format":          {"json"},
	}
	body, err := v.send(ctx, apiBaseURL, nil, form)
	if err != nil {
		return nil, fmt.Errorf("vchat.joinConversationByLink: %w", err)
	}
	var r vchatJoinResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("vchat.joinConversationByLink parse: %w", err)
	}
	if r.TurnServer == nil {
		return nil, fmt.Errorf("vchat.joinConversationByLink: no turn_server: %s", body)
	}
	if len(r.TurnServer.URLs) == 0 || r.TurnServer.Username == "" || r.TurnServer.Credential == "" {
		return nil, fmt.Errorf("vchat.joinConversationByLink: incomplete turn_server")
	}
	expiresAt, err := parseExpiryFromUsername(r.TurnServer.Username)
	if err != nil {
		return nil, fmt.Errorf("vchat.joinConversationByLink: %w", err)
	}
	r.TurnServer.ExpiresAt = expiresAt
	return &VChatJoinResponse{TurnServer: *r.TurnServer}, nil
}

func normalizeFbDo(apiBase string) string {
	trimmed := strings.TrimRight(apiBase, "/")
	if strings.HasSuffix(trimmed, "/fb.do") {
		return trimmed
	}
	return trimmed + "/fb.do"
}

func randomDeviceSuffix() string {
	return fmt.Sprintf("%08x", rand.Uint32())
}
