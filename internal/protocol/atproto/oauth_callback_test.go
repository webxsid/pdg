package atproto

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCallbackServer(t *testing.T) {
	server, err := newCallbackServer()
	if err != nil {
		t.Fatalf(
			"newCallbackServer() unexpected error = %v",
			err,
		)
	}

	server.start()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			time.Second,
		)
		defer cancel()

		_ = server.close(ctx)
	})

	callbackURL, err := url.Parse(server.redirectURI())
	if err != nil {
		t.Fatal(err)
	}

	query := callbackURL.Query()
	query.Set("code", "test-code")
	query.Set("state", "test-state")

	callbackURL.RawQuery = query.Encode()

	resp, err := http.Get(callbackURL.String())
	if err != nil {
		t.Fatalf(
			"GET callback: %v",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf(
			"status = %d, want %d",
			resp.StatusCode,
			http.StatusOK,
		)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Second,
	)
	defer cancel()

	result, err := server.wait(ctx)
	if err != nil {
		t.Fatalf(
			"wait() unexpected error = %v",
			err,
		)
	}

	if result.Code != "test-code" {
		t.Errorf(
			"Code = %q, want %q",
			result.Code,
			"test-code",
		)
	}

	if result.State != "test-state" {
		t.Errorf(
			"State = %q, want %q",
			result.State,
			"test-state",
		)
	}
}

func TestCallbackServerRejectsInvalidCallback(t *testing.T) {
	tests := []struct {
		name  string
		query url.Values
	}{
		{
			name: "missing code",
			query: url.Values{
				"state": {"test-state"},
			},
		},
		{
			name: "missing state",
			query: url.Values{
				"code": {"test-code"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := newCallbackServer()
			if err != nil {
				t.Fatal(err)
			}

			server.start()

			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(
					context.Background(),
					time.Second,
				)
				defer cancel()

				_ = server.close(ctx)
			})

			callbackURL, err := url.Parse(server.redirectURI())
			if err != nil {
				t.Fatal(err)
			}

			callbackURL.RawQuery = tt.query.Encode()

			resp, err := http.Get(
				callbackURL.String(),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode !=
				http.StatusBadRequest {
				t.Errorf(
					"status = %d, want %d",
					resp.StatusCode,
					http.StatusBadRequest,
				)
			}

			ctx, cancel := context.WithTimeout(
				context.Background(),
				time.Second,
			)
			defer cancel()

			_, err = server.wait(ctx)

			if err == nil {
				t.Fatal(
					"wait() error = nil, want error",
				)
			}
		})
	}
}

func TestCallbackServerReportsOAuthError(t *testing.T) {
	server, err := newCallbackServer()
	if err != nil {
		t.Fatalf("newCallbackServer() unexpected error = %v", err)
	}
	server.start()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.close(ctx)
	})

	resp, err := http.Get(server.redirectURI() + "?error=access_denied&error_description=user+denied+authorization")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = server.wait(ctx)
	if err == nil {
		t.Fatal("wait() error = nil, want authorization error")
	}
	if err.Error() != "authorization failed: access_denied: user denied authorization" {
		t.Errorf("wait() error = %q, want OAuth error", err)
	}
}

func TestCallbackServerSuccessResponse(t *testing.T) {
	server, err := newCallbackServer()
	if err != nil {
		t.Fatal(err)
	}

	server.start()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			time.Second,
		)
		defer cancel()

		_ = server.close(ctx)
	})

	resp, err := http.Get(
		server.redirectURI() +
			"?code=test-code&state=test-state",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(
		string(body),
		"Authorization complete",
	) {
		t.Errorf(
			"unexpected callback response: %q",
			string(body),
		)
	}
}

func TestCallbackServerWaitCancellation(t *testing.T) {
	server, err := newCallbackServer()
	if err != nil {
		t.Fatal(err)
	}

	server.start()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			time.Second,
		)
		defer cancel()

		_ = server.close(ctx)
	})

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	cancel()

	_, err = server.wait(ctx)

	if err == nil {
		t.Fatal(
			"wait() error = nil, want context error",
		)
	}

	if err != context.Canceled {
		t.Errorf(
			"error = %v, want context.Canceled",
			err,
		)
	}
}
