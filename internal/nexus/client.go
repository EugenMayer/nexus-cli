// Package nexus is a minimal client for the Nexus 3 repository REST API,
// scoped to the operations the CLI needs.
package nexus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Asset is a single asset in a repository.
type Asset struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type assetsResponse struct {
	Items             []Asset `json:"items"`
	ContinuationToken string  `json:"continuationToken"`
}

// Client talks to a single Nexus instance.
type Client struct {
	base string
	user string
	pass string
	http *http.Client
}

// New returns a Client. baseURL trailing slashes are trimmed. user/pass may be
// empty for anonymous access. timeout is applied per request.
func New(baseURL, user, pass string, timeout time.Duration) *Client {
	return &Client{
		base: strings.TrimRight(baseURL, "/"),
		user: user,
		pass: pass,
		http: &http.Client{Timeout: timeout},
	}
}

func (c *Client) do(ctx context.Context, method, u string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, err
	}
	if c.user != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
	req.Header.Set("Accept", "application/json")
	return c.http.Do(req)
}

// ListAssets pages through every asset in the repository and returns them all.
func (c *Client) ListAssets(ctx context.Context, repo string) ([]Asset, error) {
	var out []Asset
	token := ""
	for {
		u := fmt.Sprintf("%s/service/rest/v1/assets?repository=%s",
			c.base, url.QueryEscape(repo))
		if token != "" {
			u += "&continuationToken=" + url.QueryEscape(token)
		}
		resp, err := c.do(ctx, http.MethodGet, u)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			msg := readErr(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("list assets: HTTP %d: %s", resp.StatusCode, msg)
		}
		var r assetsResponse
		err = json.NewDecoder(resp.Body).Decode(&r)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decode assets response: %w", err)
		}
		out = append(out, r.Items...)
		if r.ContinuationToken == "" {
			return out, nil
		}
		token = r.ContinuationToken
	}
}

// DeleteAsset deletes a single asset by its ID.
func (c *Client) DeleteAsset(ctx context.Context, id string) error {
	u := fmt.Sprintf("%s/service/rest/v1/assets/%s", c.base, url.PathEscape(id))
	resp, err := c.do(ctx, http.MethodDelete, u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, readErr(resp.Body))
	}
	return nil
}

func readErr(r io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(r, 1024))
	return strings.TrimSpace(string(b))
}

// ErrUnauthorized is returned by CheckAuth when Nexus rejects the credentials.
var ErrUnauthorized = errors.New("invalid credentials")

// CheckAuth verifies the client's credentials with a cheap authenticated
// request. A 403 counts as success: the credentials are valid, the user just
// lacks the privilege to browse repositories.
func (c *Client) CheckAuth(ctx context.Context) error {
	resp, err := c.do(ctx, http.MethodGet, c.base+"/service/rest/v1/repositories")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusForbidden:
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	default:
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, readErr(resp.Body))
	}
}
