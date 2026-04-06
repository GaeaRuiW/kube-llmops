package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDCProvider struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
}

func NewOIDCProvider(issuerURL, clientID, clientSecret, redirectURL string) (*OIDCProvider, error) {
	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})
	oauthCfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	return &OIDCProvider{provider: provider, verifier: verifier, oauth: oauthCfg}, nil
}

func (o *OIDCProvider) Verify(ctx context.Context, rawToken string) (*oidc.IDToken, error) {
	return o.verifier.Verify(ctx, rawToken)
}

func (o *OIDCProvider) AuthCodeURL(state string) string {
	return o.oauth.AuthCodeURL(state)
}

func (o *OIDCProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return o.oauth.Exchange(ctx, code)
}
