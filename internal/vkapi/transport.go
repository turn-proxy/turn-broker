package vkapi

import (
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

func newChromeClient(jar tlsclient.CookieJar) (tlsclient.HttpClient, error) {
	return tlsclient.NewHttpClient(tlsclient.NewNoopLogger(),
		tlsclient.WithClientProfile(profiles.Chrome_146),
		tlsclient.WithTimeoutSeconds(30),
		tlsclient.WithCookieJar(jar),
	)
}
