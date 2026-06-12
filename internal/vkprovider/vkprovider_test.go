package vkprovider

import (
	"errors"
	"fmt"
	"testing"

	"github.com/turn-proxy/turn-broker/internal/turn"
	"github.com/turn-proxy/turn-broker/internal/vkapi"
)

func TestClassifyFetchError(t *testing.T) {
	authErr := fmt.Errorf("auth.anonymLogin: %w", &vkapi.APIError{ErrorCode: vkapi.AuthFailed})
	if !errors.Is(classifyFetchError(authErr), turn.ErrAuthExpired) {
		t.Fatal("auth-failed code should map to ErrAuthExpired")
	}

	gone := fmt.Errorf("vchat.joinConversationByLink: %w", &vkapi.APIError{ErrorCode: 1114, ErrorMsg: "join_link.decode_error"})
	if !errors.Is(classifyFetchError(gone), turn.ErrCallGone) {
		t.Fatal("non-transient code (1114) should map to ErrCallGone")
	}

	rateLimited := fmt.Errorf("wrap: %w", &vkapi.APIError{ErrorCode: vkapi.TooManyRequests})
	got := classifyFetchError(rateLimited)
	if errors.Is(got, turn.ErrCallGone) || errors.Is(got, turn.ErrAuthExpired) {
		t.Fatal("transient code should pass through unclassified")
	}

	network := errors.New("dial tcp: i/o timeout")
	if classifyFetchError(network) != network {
		t.Fatal("non-VK error should be returned unchanged")
	}
}
