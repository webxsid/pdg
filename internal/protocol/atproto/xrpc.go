package atproto

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// SessionStore provides the persisted session operations needed by XRPC.
type SessionStore interface {
	LoadSession(context.Context, string) (Session, error)
	LoadActiveSession(context.Context) (Session, error)
	RefreshSession(context.Context, *Client, string) (Session, error)
	SaveSession(context.Context, Session, bool) error
}

// XRPCClient sends authenticated ATProto XRPC requests for the active account.
type XRPCClient struct {
	httpClient *http.Client
	authClient *Client
	sessions   SessionStore
}

func NewXRPCClient(httpClient *http.Client, sessions SessionStore) *XRPCClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &XRPCClient{httpClient: httpClient, authClient: NewClient(httpClient), sessions: sessions}
}

// Do sends an authenticated XRPC request and decodes its JSON response.
func (c *XRPCClient) Do(ctx context.Context, method, nsid string, body any, result any) error {
	if c.sessions == nil {
		return errors.New("XRPC session store is nil")
	}
	session, err := c.sessions.LoadActiveSession(ctx)
	if err != nil {
		return fmt.Errorf("load active ATProto session: %w", err)
	}
	return c.do(ctx, session, method, nsid, body, result, false, false)
}

// DoWithSession sends an authenticated request using the supplied explicit session.
func (c *XRPCClient) DoWithSession(ctx context.Context, session Session, method, nsid string, body any, result any) error {
	return c.do(ctx, session, method, nsid, body, result, false, false)
}

// CreateRecord creates an ATProto record using the server-generated rkey.
func (c *XRPCClient) CreateRecord(ctx context.Context, session Session, collection string, record any) (string, error) {
	result, err := c.CreateRecordResult(ctx, session, collection, record)
	return result.URI, err
}

type RecordResult struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

func (c *XRPCClient) CreateRecordResult(ctx context.Context, session Session, collection string, record any) (RecordResult, error) {
	var result struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	if err := c.DoWithSession(ctx, session, http.MethodPost, "com.atproto.repo.createRecord", map[string]any{
		"repo": session.Identity.DID, "collection": collection, "record": record,
	}, &result); err != nil {
		return RecordResult{}, err
	}
	if result.URI == "" {
		return RecordResult{}, fmt.Errorf("createRecord response missing uri")
	}
	return RecordResult{URI: result.URI, CID: result.CID}, nil
}

// UpdateRecord updates an existing record and returns its new CID.
func (c *XRPCClient) UpdateRecord(ctx context.Context, session Session, collection, rkey string, record any) (string, error) {
	var result struct {
		CID string `json:"cid"`
	}
	if err := c.DoWithSession(ctx, session, http.MethodPost, "com.atproto.repo.putRecord", map[string]any{
		"repo": session.Identity.DID, "collection": collection, "rkey": rkey, "record": record,
	}, &result); err != nil {
		return "", err
	}
	return result.CID, nil
}

func (c *XRPCClient) do(ctx context.Context, session Session, method, nsid string, body, result any, nonceRetried, refreshed bool) error {
	endpoint, err := xrpcEndpoint(session.Identity.PDS, nsid)
	if err != nil {
		return err
	}
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode XRPC request: %w", err)
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, payload)
	if err != nil {
		return fmt.Errorf("create XRPC request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	proof, err := session.dpopKey.resourceProof(method, endpoint, session.DPoPNonce, session.AccessToken)
	if err != nil {
		return fmt.Errorf("create XRPC DPoP proof: %w", err)
	}
	req.Header.Set("Authorization", "DPoP "+session.AccessToken)
	req.Header.Set("DPoP", proof)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send XRPC request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read XRPC response: %w", err)
	}
	responseNonce := resp.Header.Get("DPoP-Nonce")
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseErr := xrpcResponseError(resp, data)
		if isDPoPNonceChallenge(resp, responseErr) && responseNonce != "" && !nonceRetried {
			session.DPoPNonce = responseNonce
			return c.do(ctx, session, method, nsid, body, result, true, refreshed)
		}
		if isTokenFailure(resp, responseErr) && !refreshed && session.RefreshToken != "" {
			updated, err := c.sessions.RefreshSession(ctx, c.authClient, session.Identity.DID)
			if err != nil {
				return fmt.Errorf("refresh ATProto session after XRPC authentication failure: %w", err)
			}
			return c.do(ctx, updated, method, nsid, body, result, false, true)
		}
		return responseErr
	}
	if responseNonce != "" && responseNonce != session.DPoPNonce {
		session.DPoPNonce = responseNonce
		if err := c.sessions.SaveSession(ctx, session, false); err != nil {
			return fmt.Errorf("persist XRPC DPoP nonce: %w", err)
		}
	}
	if result != nil && len(data) > 0 {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("decode XRPC response: %w", err)
		}
	}
	return nil
}

func xrpcEndpoint(pds, nsid string) (string, error) {
	parsed, err := url.Parse(pds)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid XRPC PDS endpoint")
	}
	if nsid == "" || strings.ContainsAny(nsid, "/?#") {
		return "", fmt.Errorf("invalid XRPC NSID %q", nsid)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/xrpc/" + nsid
	parsed.RawQuery, parsed.Fragment = "", ""
	return parsed.String(), nil
}

type xrpcError struct {
	Status int
	Code   string `json:"error"`
	Msg    string `json:"message"`
}

func (e *xrpcError) Error() string {
	if e.Code != "" || e.Msg != "" {
		return fmt.Sprintf("XRPC request failed: %s: %s", e.Code, e.Msg)
	}
	return fmt.Sprintf("XRPC request failed with status %d", e.Status)
}

func xrpcResponseError(resp *http.Response, data []byte) error {
	result := &xrpcError{Status: resp.StatusCode}
	_ = json.Unmarshal(data, result)
	return result
}

func isDPoPNonceChallenge(resp *http.Response, err error) bool {
	return strings.Contains(resp.Header.Get("WWW-Authenticate"), "use_dpop_nonce") || strings.Contains(err.Error(), "use_dpop_nonce")
}

func isTokenFailure(resp *http.Response, err error) bool {
	return resp.StatusCode == http.StatusUnauthorized && (strings.Contains(resp.Header.Get("WWW-Authenticate"), "invalid_token") || strings.Contains(err.Error(), "ExpiredToken") || strings.Contains(err.Error(), "InvalidToken"))
}
