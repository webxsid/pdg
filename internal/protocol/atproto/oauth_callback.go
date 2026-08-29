package atproto

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
)

type oauthCallback struct {
	Code             string `json:"code"`
	State            string `json:"state"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type callbackServer struct {
	listener net.Listener
	server   *http.Server
	result   chan callbackResult
}

type callbackResult struct {
	callback oauthCallback
	err      error
}

func newCallbackServer() (*callbackServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to start callback server: %w", err,
		)
	}

	c := &callbackServer{
		listener: listener,
		result:   make(chan callbackResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", c.handleCallback)

	c.server = &http.Server{
		Handler: mux,
	}

	return c, nil
}

func (c *callbackServer) redirectURI() string {
	address := c.listener.Addr().(*net.TCPAddr)
	return fmt.Sprintf(
		"http://%s/callback", address.String(),
	)
}

func (c *callbackServer) start() {
	go func() {
		err := c.server.Serve(c.listener)

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case c.result <- callbackResult{err: fmt.Errorf("callback server error: %w", err)}:
			default:
				// If the channel is already closed, we can't send the error.
				// This can happen if the server was closed before an error occurred.
			}
		}
	}()
}

func (c *callbackServer) handleCallback(
	w http.ResponseWriter,
	r *http.Request,
) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	oauthError := r.URL.Query().Get("error")
	errorDescription := r.URL.Query().Get("error_description")

	if oauthError != "" {
		http.Error(
			w,
			"Authorization failed: "+oauthError,
			http.StatusBadRequest,
		)

		message := "authorization failed: " + oauthError
		if errorDescription != "" {
			message += ": " + errorDescription
		}
		c.sendResult(callbackResult{err: fmt.Errorf("%s", message)})
		return
	}

	if code == "" {
		http.Error(
			w,
			"Missing authorization code",
			http.StatusBadRequest,
		)

		c.sendResult(callbackResult{
			err: fmt.Errorf("missing authorization code"),
		})
		return
	}

	if state == "" {
		http.Error(
			w,
			"Missing state parameter",
			http.StatusBadRequest,
		)

		c.sendResult(callbackResult{
			err: fmt.Errorf("missing state parameter"),
		})
		return
	}

	// Finish the HTTP response first.

	w.Header().Set(
		"Content-Type",
		"text/plain; charset=utf-8",
	)
	_, _ = io.WriteString(
		w,
		"Authorization complete. You can close this window.\n",
	)

	c.sendResult(callbackResult{
		callback: oauthCallback{
			Code:             code,
			State:            state,
			Error:            oauthError,
			ErrorDescription: errorDescription,
		},
	})
}

func (c *callbackServer) sendResult(
	result callbackResult,
) {
	select {
	case c.result <- result:
	default:
	}
}

func (c *callbackServer) wait(
	ctx context.Context,
) (oauthCallback, error) {
	select {
	case <-ctx.Done():
		return oauthCallback{}, ctx.Err()

	case result := <-c.result:
		return result.callback, result.err
	}
}

func (c *callbackServer) close(
	ctx context.Context,
) error {
	return c.server.Shutdown(ctx)
}
