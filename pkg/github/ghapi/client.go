package ghapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Client provides REST and GraphQL access to the GitHub API.
type Client struct {
	rest    *api.RESTClient
	graphql *api.GraphQLClient
	host    string
}

// ClientOptions configures the GitHub API client.
type ClientOptions struct {
	Host      string
	Transport http.RoundTripper
}

// NewClient creates a new GitHub API client.
// Call CheckAuth first to ensure credentials are available.
func NewClient(opts ClientOptions) (*Client, error) {
	host := opts.Host
	if host == "" {
		host = defaultHost
	}

	apiOpts := api.ClientOptions{
		Host: host,
	}
	if opts.Transport != nil {
		apiOpts.Transport = opts.Transport
	}

	rest, err := api.NewRESTClient(apiOpts)
	if err != nil {
		return nil, fmt.Errorf("creating REST client: %w", err)
	}

	graphql, err := api.NewGraphQLClient(apiOpts)
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}

	return &Client{
		rest:    rest,
		graphql: graphql,
		host:    host,
	}, nil
}

// Host returns the GitHub host this client is configured for.
func (c *Client) Host() string {
	return c.host
}

// REST returns the underlying REST client for direct access.
func (c *Client) REST() *api.RESTClient {
	return c.rest
}

// GraphQL returns the underlying GraphQL client for direct access.
func (c *Client) GraphQL() *api.GraphQLClient {
	return c.graphql
}

// Get performs a REST GET request.
func (c *Client) Get(ctx context.Context, path string, result interface{}) error {
	err := c.rest.DoWithContext(ctx, "GET", path, nil, result)
	return ClassifyHTTPError(err, path)
}

// GetWithHeaders performs a REST GET request and returns response headers.
func (c *Client) GetWithHeaders(ctx context.Context, path string, result interface{}) (http.Header, error) {
	resp, err := c.rest.RequestWithContext(ctx, "GET", path, nil)
	if err != nil {
		return nil, ClassifyHTTPError(err, path)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		if err := api.HandleHTTPError(resp); err != nil {
			return resp.Header, ClassifyHTTPError(err, path)
		}
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return resp.Header, fmt.Errorf("decoding response: %w", err)
		}
	}

	return resp.Header, nil
}

// Query performs a GraphQL query.
func (c *Client) Query(ctx context.Context, query string, variables map[string]interface{}, result interface{}) error {
	err := c.graphql.DoWithContext(ctx, query, variables, result)
	if err != nil {
		return ClassifyGraphQLError(err)
	}
	return nil
}
