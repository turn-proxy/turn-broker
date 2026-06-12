package vkprovider

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/turn-proxy/turn-broker/internal/turn"
	"github.com/turn-proxy/turn-broker/internal/vkapi"
	"golang.org/x/sync/errgroup"
)

var _ turn.Provider = (*Provider)(nil)

func classifyFetchError(err error) error {
	ae, ok := vkapi.AsAPIError(err)
	if !ok {
		return err
	}
	switch {
	case ae.ErrorCode == vkapi.AuthFailed:
		return fmt.Errorf("%w: %w", turn.ErrAuthExpired, err)
	case !ae.IsTransient():
		return fmt.Errorf("%w: %w", turn.ErrCallGone, err)
	}
	return err
}

type Provider struct {
	vk          *vkapi.Client
	cookiesFile string
}

func New(cookiesFile string, appID, apiVersion *string) (*Provider, error) {
	raw, err := os.ReadFile(cookiesFile)
	if err != nil {
		return nil, fmt.Errorf("reading cookies file %s: %w", cookiesFile, err)
	}
	cookies, err := vkapi.CookiesFromJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("cookies file %s: %w", cookiesFile, err)
	}
	builder := vkapi.NewBuilder(cookies)
	if appID != nil {
		builder.AppID(*appID)
	}
	if apiVersion != nil {
		builder.APIVersion(*apiVersion)
	}
	vk, err := builder.Build()
	if err != nil {
		return nil, err
	}
	return &Provider{vk: vk, cookiesFile: cookiesFile}, nil
}

func (p *Provider) Fetch(ctx context.Context, joinLink string) (turn.Creds, error) {
	callToken, err := vkapi.ExtractCallToken(joinLink)
	if err != nil {
		return turn.Creds{}, fmt.Errorf("parse join link: %w", err)
	}
	server, err := p.fetchTurn(ctx, callToken)
	if err != nil {
		return turn.Creds{}, classifyFetchError(err)
	}
	p.syncCookies()
	return turn.Creds{
		URLs:       server.URLs,
		Username:   server.Username,
		Credential: server.Credential,
		ExpiresAt:  server.ExpiresAt,
	}, nil
}

func (p *Provider) fetchTurn(ctx context.Context, callToken string) (*vkapi.VChatTurnServer, error) {
	var settings *vkapi.CallsSettings
	var call *vkapi.MessagesCallToken
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		s, err := p.vk.CallsGetSettings(gctx)
		settings = s
		return err
	})
	g.Go(func() error {
		c, err := p.vk.MessagesGetCallToken(gctx)
		call = c
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}
	sessionKey, err := p.vk.AuthAnonymLogin(ctx, call.APIBaseURL, settings.PublicKey, call.Token)
	if err != nil {
		return nil, err
	}
	join, err := p.vk.VChatJoinConversationByLink(ctx, call.APIBaseURL, settings.PublicKey, sessionKey, callToken)
	if err != nil {
		return nil, err
	}
	return &join.TurnServer, nil
}

func (p *Provider) syncCookies() {
	if err := vkapi.WriteCookiesFile(p.cookiesFile, p.vk.SnapshotCookies()); err != nil {
		slog.Warn("cookies sync failed", "path", p.cookiesFile, "err", err)
	}
}
