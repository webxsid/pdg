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

type parRequest struct {
	ClientID      string
	RedirectURI   string
	Scope         string
	LoginHint     string
	State         string
	CodeChallenge string
}

type parResponse struct {
	RequestURI string `json:"request_uri"`
	ExpiresIn  int    `json:"expires_in"`
	DPoPNonce  string `json:"dpop_nonce"`
}

type parWireResponse struct {
	RequestURI string `json:"request_uri"`
	ExpiresIn  int    `json:"expires_in"`
}

func (c *oauthClient) pushAuthorizationRequest(
	ctx context.Context,
	server AuthorizationServer,
	request parRequest,
	key *dpopKey,
) (parResponse, error) {
	return c.pushAuthorizationRequestWithNonce(
		ctx, server, request, key, "", true,
	)
}

func (c *oauthClient) pushAuthorizationRequestWithNonce(
	ctx context.Context,
	server AuthorizationServer,
	request parRequest,
	key *dpopKey,
	nonce string,
	allowRetry bool,
) (parResponse, error) {
	values := url.Values{
		"client_id":             {request.ClientID},
		"response_type":         {"code"},
		"redirect_uri":          {request.RedirectURI},
		"scope":                 {request.Scope},
		"state":                 {request.State},
		"code_challenge":        {request.CodeChallenge},
		"code_challenge_method": {"S256"},
	}

	if request.LoginHint != "" {
		values.Set("login_hint", request.LoginHint)
	}

	proof, err := key.proof(
		http.MethodPost,
		server.PushedAuthorizationRequestEndpoint,
		nonce,
	)
	if err != nil {
		return parResponse{}, fmt.Errorf(
			"create DPoP proof: %w",
			err,
		)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		server.PushedAuthorizationRequestEndpoint,
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return parResponse{}, err
	}

	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	req.Header.Set("DPoP", proof)

	resp, err := c.client.Do(req)
	if err != nil {
		return parResponse{}, fmt.Errorf(
			"send PAR request: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, readErr := io.ReadAll(
			io.LimitReader(resp.Body, 4096),
		)
		if readErr != nil {
			return parResponse{}, fmt.Errorf(
				"read PAR error response: %w", readErr,
			)
		}

		var oauthErr oauthErrorResponse
		_ = json.Unmarshal(body, &oauthErr)
		responseNonce := resp.Header.Get("DPoP-Nonce")
		if oauthErr.Error == "use_dpop_nonce" &&
			responseNonce != "" && allowRetry {
			return c.pushAuthorizationRequestWithNonce(
				ctx, server, request, key, responseNonce, false,
			)
		}

		return parResponse{}, fmt.Errorf(
			"PAR request failed: %s: %s",
			resp.Status,
			string(body),
		)
	}

	responseNonce := resp.Header.Get("DPoP-Nonce")
	if responseNonce == "" {
		return parResponse{}, fmt.Errorf(
			"PAR response missing DPoP-Nonce header",
		)
	}

	var result parWireResponse

	if err := json.NewDecoder(
		io.LimitReader(resp.Body, 1<<20),
	).Decode(&result); err != nil {
		return parResponse{}, fmt.Errorf(
			"decode PAR response: %w",
			err,
		)
	}

	if result.RequestURI == "" {
		return parResponse{}, fmt.Errorf(
			"PAR response missing request_uri",
		)
	}

	if result.ExpiresIn <= 0 {
		return parResponse{}, fmt.Errorf(
			"PAR response missing expires_in",
		)
	}

	return parResponse{
		RequestURI: result.RequestURI,
		ExpiresIn:  result.ExpiresIn,
		DPoPNonce:  responseNonce,
	}, nil
}
