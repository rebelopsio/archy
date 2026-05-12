package linear

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/domain"
)

// fakeListIssuesInput matches the keys archy sends to list_issues.
// Used as the typed input for in-memory test servers.
type fakeListIssuesInput struct {
	Assignee string `json:"assignee,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	OrderBy  string `json:"orderBy,omitempty"`
}

// fakeListIssuesOutput is the wrapped shape archy expects from
// list_issues by default.
type fakeListIssuesOutput struct {
	Issues []linearIssue `json:"issues"`
}

// startFakeLinear stands up an in-memory MCP server that registers a
// list_issues tool whose handler is supplied by the caller. Returns a
// *Client wired to that server's transport, plus a cleanup func.
func startFakeLinear(
	t *testing.T,
	handler func(ctx context.Context, req *mcp.CallToolRequest, in fakeListIssuesInput) (*mcp.CallToolResult, fakeListIssuesOutput, error),
) (*Client, func()) {
	t.Helper()

	clientT, serverT := mcp.NewInMemoryTransports()

	srv := mcp.NewServer(&mcp.Implementation{Name: "fake-linear", Version: "0.0.1"}, nil)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_issues",
		Description: "fake list_issues for in-memory tests",
	}, handler)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	serverSess, err := srv.Connect(ctx, serverT, nil)
	require.NoError(t, err)

	c := newWithTransport(clientT)
	cleanup := func() {
		_ = c.Close()
		_ = serverSess.Close()
	}
	t.Cleanup(cleanup)
	return c, cleanup
}

func TestNew_ErrConfig_MissingURL(t *testing.T) {
	_, err := New(Config{BearerToken: "tok"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrConfig))
}

func TestNew_ErrConfig_MissingToken(t *testing.T) {
	_, err := New(Config{URL: "https://mcp.linear.app/mcp"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrConfig))
}

func TestNew_OK(t *testing.T) {
	c, err := New(Config{URL: "https://mcp.linear.app/mcp", BearerToken: "tok"})
	require.NoError(t, err)
	assert.NoError(t, c.Close())
}

func TestClose_Idempotent(t *testing.T) {
	c, err := New(Config{URL: "https://mcp.linear.app/mcp", BearerToken: "tok"})
	require.NoError(t, err)
	assert.NoError(t, c.Close())
	assert.NoError(t, c.Close())
}

func TestListMyIssues_HappyPath(t *testing.T) {
	c, _ := startFakeLinear(t, func(_ context.Context, _ *mcp.CallToolRequest, in fakeListIssuesInput) (*mcp.CallToolResult, fakeListIssuesOutput, error) {
		assert.Equal(t, "me", in.Assignee)
		assert.Equal(t, 50, in.Limit)
		assert.Equal(t, "updatedAt", in.OrderBy)
		return nil, fakeListIssuesOutput{Issues: []linearIssue{
			{
				ID:         "ENG-1",
				URL:        "https://linear.app/x/issue/ENG-1",
				Title:      "First",
				StatusType: "started",
				Priority:   &linearPriority{Value: 1, Name: "Urgent"},
			},
			{
				ID:         "SOC-2",
				URL:        "https://linear.app/x/issue/SOC-2",
				Title:      "Second",
				StatusType: "unstarted",
				Priority:   &linearPriority{Value: 3, Name: "Medium"},
			},
		}}, nil
	})

	issues, err := c.ListMyIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 2)
	assert.Equal(t, "ENG-1", issues[0].Ref.ID)
	assert.Equal(t, "linear", issues[0].Ref.Provider)
	assert.Equal(t, domain.PriorityUrgent, issues[0].Priority)
	assert.Equal(t, domain.IssueStateInProgress, issues[0].State)
	assert.Equal(t, "SOC-2", issues[1].Ref.ID)
	assert.Equal(t, domain.PriorityMedium, issues[1].Priority)
	assert.Equal(t, domain.IssueStateTodo, issues[1].State)
}

func TestListMyIssues_FiltersCompletedAndCanceled(t *testing.T) {
	c, _ := startFakeLinear(t, func(_ context.Context, _ *mcp.CallToolRequest, _ fakeListIssuesInput) (*mcp.CallToolResult, fakeListIssuesOutput, error) {
		return nil, fakeListIssuesOutput{Issues: []linearIssue{
			{ID: "A", StatusType: "started"},
			{ID: "B", StatusType: "completed"},
			{ID: "C", StatusType: "canceled"},
			{ID: "D", StatusType: "backlog"},
		}}, nil
	})

	issues, err := c.ListMyIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 2, "completed and canceled should be filtered client-side")
	assert.Equal(t, "A", issues[0].Ref.ID)
	assert.Equal(t, "D", issues[1].Ref.ID)
}

func TestListMyIssues_EmptyResult(t *testing.T) {
	c, _ := startFakeLinear(t, func(_ context.Context, _ *mcp.CallToolRequest, _ fakeListIssuesInput) (*mcp.CallToolResult, fakeListIssuesOutput, error) {
		return nil, fakeListIssuesOutput{Issues: []linearIssue{}}, nil
	})
	issues, err := c.ListMyIssues(context.Background())
	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestListMyIssues_ToolErrorWrapsErrToolCall(t *testing.T) {
	c, _ := startFakeLinear(t, func(_ context.Context, _ *mcp.CallToolRequest, _ fakeListIssuesInput) (*mcp.CallToolResult, fakeListIssuesOutput, error) {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: "linear unavailable"}},
		}, fakeListIssuesOutput{}, nil
	})
	_, err := c.ListMyIssues(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrToolCall))
	assert.Contains(t, err.Error(), "linear unavailable")
}

// === parseIssues / unmarshalIssues shape variants ===

func TestUnmarshalIssues_Wrapped(t *testing.T) {
	b := []byte(`{"issues":[{"id":"ENG-1","statusType":"started"}]}`)
	got, err := unmarshalIssues(b)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "ENG-1", got[0].ID)
}

func TestUnmarshalIssues_BareArray(t *testing.T) {
	b := []byte(`[{"id":"ENG-1","statusType":"started"}]`)
	got, err := unmarshalIssues(b)
	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestUnmarshalIssues_DataAlias(t *testing.T) {
	b := []byte(`{"data":[{"id":"X-1"}]}`)
	got, err := unmarshalIssues(b)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "X-1", got[0].ID)
}

func TestUnmarshalIssues_ResultsAlias(t *testing.T) {
	b := []byte(`{"results":[{"id":"X-2"}]}`)
	got, err := unmarshalIssues(b)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "X-2", got[0].ID)
}

func TestUnmarshalIssues_EmptyWrappedReturnsEmpty(t *testing.T) {
	b := []byte(`{"issues":[]}`)
	got, err := unmarshalIssues(b)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestUnmarshalIssues_Garbage(t *testing.T) {
	_, err := unmarshalIssues([]byte("not json"))
	require.Error(t, err)
}

func TestUnmarshalIssues_GarbageIncludesBothErrorsInMessage(t *testing.T) {
	_, err := unmarshalIssues([]byte(`{"surprise":"new shape"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not parse as a known shape")
	assert.Contains(t, err.Error(), "wrapped:")
	assert.Contains(t, err.Error(), "bare:")
	assert.Contains(t, err.Error(), `{"surprise":"new shape"}`)
}

func TestUnmarshalIssues_FieldMismatchSurfacesWrappedError(t *testing.T) {
	// `priority` is supposed to decode into *linearPriority (an object),
	// but here we send it as a bare integer. The wrapped decode will
	// fail with a type-mismatch error; we want that error visible.
	b := []byte(`{"issues":[{"id":"ENG-1","priority":1}]}`)
	_, err := unmarshalIssues(b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wrapped:")
	// The Go json package's error message for this case includes
	// "cannot unmarshal number into Go" or similar; assert the
	// substring "cannot unmarshal" which is stable across versions.
	assert.Contains(t, err.Error(), "cannot unmarshal")
}

func TestUnmarshalIssues_DumpsFullBodyOnFailure(t *testing.T) {
	body := []byte(`{"surprise":"new shape"}`)
	_, err := unmarshalIssues(body)
	require.Error(t, err)

	// Extract the dump path from the error message. The message format
	// is stable: "...; full body written to <path>".
	msg := err.Error()
	const marker = "full body written to "
	idx := strings.Index(msg, marker)
	require.GreaterOrEqual(t, idx, 0, "error should mention the dump path: %s", msg)
	path := strings.TrimSpace(msg[idx+len(marker):])

	t.Cleanup(func() { _ = os.Remove(path) })

	got, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, body, got)
}

func TestUnmarshalIssues_LongBodyTruncated(t *testing.T) {
	big := make([]byte, 700)
	for i := range big {
		big[i] = 'x'
	}
	body := append([]byte(`{"weird":"`), big...)
	body = append(body, []byte(`"}`)...)

	_, err := unmarshalIssues(body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not parse as a known shape")
	assert.Contains(t, err.Error(), "more bytes")
	// The inline preview must remain bounded — the message can grow
	// from the decode-error strings and the temp-file path, but the
	// 700-byte payload must not be inlined in full.
	assert.NotContains(t, err.Error(), strings.Repeat("x", 600))
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", truncate([]byte("abc"), 10))
	assert.Equal(t, "abc", truncate([]byte("abc"), 3))
	got := truncate([]byte("abcdefghij"), 5)
	assert.Equal(t, "abcde…(5 more bytes)", got)
}
