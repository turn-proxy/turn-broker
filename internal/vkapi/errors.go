package vkapi

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	UnknownError    int64 = 1
	AuthFailed      int64 = 5
	TooManyRequests int64 = 6
	FloodControl    int64 = 9
	InternalError   int64 = 10
)

var transientCodes = map[int64]bool{
	UnknownError:    true,
	TooManyRequests: true,
	FloodControl:    true,
	InternalError:   true,
}

type APIError struct {
	ErrorCode int64  `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("vk api error %d: %s", e.ErrorCode, e.ErrorMsg)
}

func (e *APIError) IsTransient() bool {
	return transientCodes[e.ErrorCode]
}

func AsAPIError(err error) (*APIError, bool) {
	return errors.AsType[*APIError](err)
}

func apiErrorFromBody(body []byte) (*APIError, bool) {
	var ae APIError
	if err := json.Unmarshal(body, &ae); err == nil && ae.ErrorCode != 0 {
		return &ae, true
	}
	return nil, false
}
