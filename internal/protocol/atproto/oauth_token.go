package atproto

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type tokenResponse struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Scope        string
	Subject      string
	DPoPNonce    string
}

type tokenWireResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	Subject      string `json:"sub"`
}

type oauthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (c *oauthClient) exchangeAuthorizationCode(
	ctx context.Context,
	server AuthorizationServer,
	authorization authorizationResult,
) (tokenResponse, error) {
	return c.exchangeAuthorizationCodeWithNonce(
		ctx,
		server,
		authorization,
		authorization.DPoPNonce,
		true,
	)
}

func (c *oauthClient) exchangeAuthorizationCodeWithNonce(
	ctx context.Context,
	server AuthorizationServer,
	authorization authorizationResult,
	nonce string,
	allowRetry bool,
) (tokenResponse, error) {
	if authorization.DPoPKey == nil {
		return tokenResponse{}, fmt.Errorf(
			"authorization result missing DPoP key",
		)
	}

	values := url.Values{}

	values.Set(
		"grant_type",
		"authorization_code",
	)

	values.Set(
		"code",
		authorization.Code,
	)

	values.Set(
		"code_verifier",
		authorization.CodeVerifier,
	)

	values.Set(
		"redirect_uri",
		authorization.RedirectURI,
	)

	values.Set(
		"client_id",
		authorization.ClientID,
	)

	proof, err := authorization.DPoPKey.proof(
		http.MethodPost,
		server.TokenEndpoint,
		nonce,
	)
	if err != nil {
		return tokenResponse{}, fmt.Errorf(
			"create token DPoP proof: %w",
			err,
		)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		server.TokenEndpoint,
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return tokenResponse{}, fmt.Errorf(
			"create token request: %w",
			err,
		)
	}

	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	req.Header.Set(
		"DPoP",
		proof,
	)

	resp, err := c.client.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf(
			"send token request: %w",
			err,
		)
	}
	defer resp.Body.Close()

	responseNonce := resp.Header.Get("DPoP-Nonce")

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(
			io.LimitReader(resp.Body, 4096),
		)
		if err != nil {
			return tokenResponse{}, fmt.Errorf(
				"read token error response: %w",
				err,
			)
		}

		var oauthErr oauthErrorResponse

		_ = json.Unmarshal(body, &oauthErr)

		if oauthErr.Error == "use_dpop_nonce" &&
			responseNonce != "" &&
			allowRetry {

			return c.exchangeAuthorizationCodeWithNonce(
				ctx,
				server,
				authorization,
				responseNonce,
				false,
			)
		}

		return tokenResponse{}, fmt.Errorf(
			"token request failed: %s: %s",
			resp.Status,
			string(body),
		)
	}

	if responseNonce == "" {
		return tokenResponse{}, fmt.Errorf(
			"token response missing DPoP-Nonce",
		)
	}

	var result tokenWireResponse

	if err := json.NewDecoder(
		io.LimitReader(resp.Body, 1<<20),
	).Decode(&result); err != nil {
		return tokenResponse{}, fmt.Errorf(
			"decode token response: %w",
			err,
		)
	}

	if result.AccessToken == "" {
		return tokenResponse{}, fmt.Errorf(
			"token response missing access_token",
		)
	}

	if result.TokenType != "DPoP" {
		return tokenResponse{}, fmt.Errorf(
			"unexpected token type %q",
			result.TokenType,
		)
	}

	if result.Subject == "" {
		return tokenResponse{}, fmt.Errorf(
			"token response missing subject",
		)
	}

	if result.Subject != authorization.Subject {
		return tokenResponse{}, fmt.Errorf(
			"token response subject %q does not match authorization subject %q",
			result.Subject,
			authorization.Subject,
		)
	}

	return tokenResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    result.TokenType,
		Scope:        result.Scope,
		Subject:      result.Subject,
		DPoPNonce:    responseNonce,
	}, nil
}
