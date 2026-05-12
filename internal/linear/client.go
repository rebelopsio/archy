package linear

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rebelopsio/archy/internal/domain"
)

// httpTimeout caps each outbound HTTP request to Linear.
const httpTimeout = 30 * time.Second

// Client is a Linear MCP client. The MCP session is opened lazily on
// the first call. Safe for concurrent use after construction.
type Client struct {
	cfg       Config
	transport mcp.Transport

	mu      sync.Mutex
	session *mcp.ClientSession
	closed  bool
}

// Config configures a [Client].
type Config struct {
	// URL is the Linear MCP endpoint (e.g. https://mcp.linear.app/mcp).
	// Required.
	URL string

	// BearerToken is the Linear PAT or OAuth access token. Required.
	BearerToken string
}

// New constructs a [Client] using the streamable HTTP transport with
// a static-bearer authorization wrapper. Returns [ErrConfig] (wrapped)
// when cfg is missing required fields.
func New(cfg Config) (*Client, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("%w: URL is required", ErrConfig)
	}
	if cfg.BearerToken == "" {
		return nil, fmt.Errorf("%w: BearerToken is required", ErrConfig)
	}
	transport := &mcp.StreamableClientTransport{
		Endpoint: cfg.URL,
		HTTPClient: &http.Client{
			Transport: &bearerRoundTripper{
				inner: http.DefaultTransport,
				token: cfg.BearerToken,
			},
			Timeout: httpTimeout,
		},
	}
	return &Client{cfg: cfg, transport: transport}, nil
}

// newWithTransport is the test-only constructor. Tests pass an
// in-memory transport pair (from [mcp.NewInMemoryTransports]) so they
// can stand up a fake server without spinning up HTTP.
func newWithTransport(transport mcp.Transport) *Client {
	return &Client{transport: transport}
}

// Close releases the MCP session if open. Safe to call multiple times.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.session != nil {
		err := c.session.Close()
		c.session = nil
		return err
	}
	return nil
}

// ensureSession opens the session lazily. Subsequent calls reuse the
// open session. Returns an error if the client has been closed.
func (c *Client) ensureSession(ctx context.Context) (*mcp.ClientSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("linear: client is closed")
	}
	if c.session != nil {
		return c.session, nil
	}
	mcpClient := mcp.NewClient(&mcp.Implementation{
		Name:    "archy-linear",
		Version: "0.1.0",
	}, nil)
	sess, err := mcpClient.Connect(ctx, c.transport, nil)
	if err != nil {
		return nil, fmt.Errorf("linear: connect: %w", err)
	}
	c.session = sess
	return sess, nil
}

// ListMyIssues returns the calling user's open assigned issues. Issues
// in completed or canceled states are filtered client-side. Linear's
// list_issues tool does not paginate the v1 50-issue cap; users with
// more than 50 open assigned issues will see a truncated list. This
// is a known limitation; pagination is deferred to v2.
//
// The reference time for "now" is the caller's; this method does not
// inspect the clock. Sorting is the caller's responsibility — the
// underlying tool's `orderBy: updatedAt` provides the wire-side order
// but downstream scoring re-ranks.
func (c *Client) ListMyIssues(ctx context.Context) ([]domain.Issue, error) {
	sess, err := c.ensureSession(ctx)
	if err != nil {
		return nil, err
	}
	res, err := sess.CallTool(ctx, &mcp.CallToolParams{
		Name: "list_issues",
		Arguments: map[string]any{
			"assignee": "me",
			"limit":    50,
			"orderBy":  "updatedAt",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("%w: list_issues: %v", ErrToolCall, err)
	}
	if res.IsError {
		return nil, fmt.Errorf("%w: list_issues: %s", ErrToolCall, toolErrorText(res))
	}
	return parseIssues(res)
}

// GatherIssues is an alias for [ListMyIssues] so *Client satisfies
// the cmd/daily IssueGatherer interface without naming-coupling. New
// providers can implement the same one-method interface.
func (c *Client) GatherIssues(ctx context.Context) ([]domain.Issue, error) {
	return c.ListMyIssues(ctx)
}

// bearerRoundTripper sets Authorization: Bearer on every outbound
// request. The token is held in memory for the client's lifetime;
// neither the wrapper nor stdlib net/http logs request headers.
type bearerRoundTripper struct {
	inner http.RoundTripper
	token string
}

// RoundTrip implements [http.RoundTripper]. Per the http package's
// contract, the request is cloned before the header is mutated to
// avoid races with concurrent retransmissions inside the transport.
func (b *bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header.Set("Authorization", "Bearer "+b.token)
	return b.inner.RoundTrip(cloned)
}

// toolErrorText extracts a human-readable error message from a
// CallToolResult that is marked IsError. Returns a fallback string
// if no text content is found.
func toolErrorText(res *mcp.CallToolResult) string {
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			return t.Text
		}
	}
	return "tool returned error with no content"
}

// parseIssues converts a CallToolResult from list_issues into a
// filtered slice of [domain.Issue]. Completed and canceled issues are
// filtered client-side; everything else is converted via
// issueFromLinear and returned in the order Linear yielded them.
func parseIssues(res *mcp.CallToolResult) ([]domain.Issue, error) {
	raw, err := extractIssuesJSON(res)
	if err != nil {
		return nil, err
	}
	issues, err := unmarshalIssues(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParse, err)
	}
	out := make([]domain.Issue, 0, len(issues))
	for _, li := range issues {
		st := stateFromLinear(li.StatusType)
		if st == domain.IssueStateDone || st == domain.IssueStateCanceled {
			continue
		}
		out = append(out, issueFromLinear(li))
	}
	return out, nil
}

// extractIssuesJSON returns the JSON bytes that should contain the
// issues array. Tries StructuredContent first (the typed-output path),
// then falls back to the first TextContent block.
func extractIssuesJSON(res *mcp.CallToolResult) ([]byte, error) {
	if res.StructuredContent != nil {
		b, err := json.Marshal(res.StructuredContent)
		if err == nil && len(b) > 0 {
			return b, nil
		}
	}
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			return []byte(t.Text), nil
		}
	}
	return nil, fmt.Errorf("%w: list_issues response had no parseable content (structured_content=%v, content_blocks=%d)", ErrParse, res.StructuredContent != nil, len(res.Content))
}

// unmarshalIssues handles both wrapped responses (the typical typed-
// output shape `{"issues": [...]}`) and bare arrays. Order: try
// wrapped first; if no wrapping field is set, fall through to bare.
// Returns the parsed slice and never nil-but-no-error so callers can
// distinguish "no issues" from "parse failed."
func unmarshalIssues(b []byte) ([]linearIssue, error) {
	var wrapped struct {
		Issues  *[]linearIssue `json:"issues,omitempty"`
		Data    *[]linearIssue `json:"data,omitempty"`
		Results *[]linearIssue `json:"results,omitempty"`
	}
	if err := json.Unmarshal(b, &wrapped); err == nil {
		if wrapped.Issues != nil {
			return *wrapped.Issues, nil
		}
		if wrapped.Data != nil {
			return *wrapped.Data, nil
		}
		if wrapped.Results != nil {
			return *wrapped.Results, nil
		}
	}
	var direct []linearIssue
	if err := json.Unmarshal(b, &direct); err == nil {
		return direct, nil
	}
	return nil, fmt.Errorf("response was neither a wrapped object nor a bare array of issues; got: %s", truncate(b, 500))
}

// truncate returns b as a string, clipped to n bytes with a trailing
// "…(N more bytes)" marker when truncation occurred. Used for keeping
// diagnostic error messages bounded when including raw response bodies.
func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + fmt.Sprintf("…(%d more bytes)", len(b)-n)
}
