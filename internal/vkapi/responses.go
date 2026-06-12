package vkapi

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type apiResponse[T any] struct {
	Response *T        `json:"response"`
	Error    *APIError `json:"error"`
}

func decodeResponse[T any](method string, body []byte) (*T, error) {
	var r apiResponse[T]
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("%s parse: %w", method, err)
	}
	if r.Error != nil {
		return nil, fmt.Errorf("%s: %w", method, r.Error)
	}
	if r.Response == nil {
		return nil, fmt.Errorf("%s: empty response", method)
	}
	return r.Response, nil
}

type webTokenData struct {
	AccessToken string `json:"access_token"`
	Expires     uint64 `json:"expires"`
}

type webTokenResponse struct {
	Data      *webTokenData `json:"data"`
	Type      string        `json:"type"`
	ErrorInfo *string       `json:"error_info"`
}

type CallsSettings struct {
	PublicKey string `json:"public_key"`
}

type callsGetSettingsResult struct {
	Settings CallsSettings `json:"settings"`
}

type MessagesCallToken struct {
	Token      string `json:"token"`
	APIBaseURL string `json:"api_base_url"`
}

type anonymLoginResponse struct {
	SessionKey string `json:"session_key"`
}

type VChatTurnServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username"`
	Credential string   `json:"credential"`
	ExpiresAt  uint64   `json:"-"`
}

type VChatJoinResponse struct {
	TurnServer VChatTurnServer
}

type vchatJoinResponse struct {
	TurnServer *VChatTurnServer `json:"turn_server"`
	Endpoint   *string          `json:"endpoint"`
}

func parseExpiryFromUsername(username string) (uint64, error) {
	prefix, _, ok := strings.Cut(username, ":")
	if !ok {
		return 0, fmt.Errorf("cannot parse expiry from username %q", username)
	}
	v, err := strconv.ParseUint(prefix, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse expiry from username %q: %w", username, err)
	}
	return v, nil
}
