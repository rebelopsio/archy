package render

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rebelopsio/archy/internal/blocks"
)

// scriptedBlock is a fake block for renderer tests. It records the
// PriorBlockData it sees on Gather (so renderer tests can assert that
// later blocks see earlier blocks' data) and yields scripted output.
type scriptedBlock struct {
	name      string
	available bool
	gather    func(gctx blocks.GatherContext) (blocks.BlockData, error)
	render    func(data blocks.BlockData) (string, error)
}

func (b scriptedBlock) Name() string { return b.name }
func (b scriptedBlock) Available(_ context.Context, _ blocks.GatherContext) bool {
	return b.available
}
func (b scriptedBlock) Gather(_ context.Context, gctx blocks.GatherContext) (blocks.BlockData, error) {
	return b.gather(gctx)
}
func (b scriptedBlock) Render(_ context.Context, data blocks.BlockData) (string, error) {
	return b.render(data)
}

// makeBlock returns a scriptedBlock that yields a fixed string and
// passes a fixed BlockData through Gather.
func makeBlock(name string, available bool, output string, data blocks.BlockData) scriptedBlock {
	return scriptedBlock{
		name:      name,
		available: available,
		gather:    func(blocks.GatherContext) (blocks.BlockData, error) { return data, nil },
		render:    func(blocks.BlockData) (string, error) { return output, nil },
	}
}

func TestRender_WalksBlocksInTemplateOrder(t *testing.T) {
	reg := blocks.NewRegistry()
	require.NoError(t, reg.Register(makeBlock("alpha", true, "## A", "data-a")))
	require.NoError(t, reg.Register(makeBlock("bravo", true, "## B", "data-b")))
	require.NoError(t, reg.Register(makeBlock("charlie", true, "## C", "data-c")))

	tpl := &Template{
		Name:      "daily-brief",
		WriteMode: "marker-block",
		MarkerID:  "x",
		Blocks: []BlockSpec{
			{Name: "charlie"},
			{Name: "alpha"},
			{Name: "bravo"},
		},
	}
	r := NewRenderer(reg)
	res, err := r.Render(context.Background(), tpl, blocks.GatherContext{Now: time.Now()})
	require.NoError(t, err)
	assert.Equal(t, "## C\n\n## A\n\n## B", res.Body)
}

func TestRender_SkipsUnavailableBlocks(t *testing.T) {
	reg := blocks.NewRegistry()
	require.NoError(t, reg.Register(makeBlock("alpha", true, "## A", nil)))
	require.NoError(t, reg.Register(makeBlock("bravo", false, "## B", nil)))
	require.NoError(t, reg.Register(makeBlock("charlie", true, "## C", nil)))

	tpl := &Template{
		Name: "daily-brief", WriteMode: "marker-block", MarkerID: "x",
		Blocks: []BlockSpec{{Name: "alpha"}, {Name: "bravo"}, {Name: "charlie"}},
	}
	r := NewRenderer(reg)
	res, err := r.Render(context.Background(), tpl, blocks.GatherContext{Now: time.Now()})
	require.NoError(t, err)
	assert.Equal(t, "## A\n\n## C", res.Body)

	require.Len(t, res.BlockResults, 3)
	assert.True(t, res.BlockResults[1].Skipped)
	assert.False(t, res.BlockResults[1].Available)
}

func TestRender_ContinuesPastBlockErrors(t *testing.T) {
	reg := blocks.NewRegistry()
	require.NoError(t, reg.Register(makeBlock("alpha", true, "## A", nil)))
	bad := scriptedBlock{
		name:      "bravo",
		available: true,
		gather: func(blocks.GatherContext) (blocks.BlockData, error) {
			return nil, errors.New("intentional gather failure")
		},
		render: func(blocks.BlockData) (string, error) { return "", nil },
	}
	require.NoError(t, reg.Register(bad))
	require.NoError(t, reg.Register(makeBlock("charlie", true, "## C", nil)))

	tpl := &Template{
		Name: "daily-brief", WriteMode: "marker-block", MarkerID: "x",
		Blocks: []BlockSpec{{Name: "alpha"}, {Name: "bravo"}, {Name: "charlie"}},
	}
	r := NewRenderer(reg)
	res, err := r.Render(context.Background(), tpl, blocks.GatherContext{Now: time.Now()})
	require.Error(t, err, "renderer reports the error")
	assert.Contains(t, err.Error(), "intentional gather failure")
	assert.Equal(t, "## A\n\n## C", res.Body, "partial body returned")

	require.Len(t, res.BlockResults, 3)
	assert.NotNil(t, res.BlockResults[1].Error)
}

func TestRender_JoinsBlockOutputsWithBlankLine(t *testing.T) {
	reg := blocks.NewRegistry()
	require.NoError(t, reg.Register(makeBlock("alpha", true, "## A\n\n- item 1\n- item 2", nil)))
	require.NoError(t, reg.Register(makeBlock("bravo", true, "## B\n\n- item 3", nil)))

	tpl := &Template{
		Name: "x", WriteMode: "marker-block", MarkerID: "x",
		Blocks: []BlockSpec{{Name: "alpha"}, {Name: "bravo"}},
	}
	r := NewRenderer(reg)
	res, err := r.Render(context.Background(), tpl, blocks.GatherContext{Now: time.Now()})
	require.NoError(t, err)
	assert.Equal(t, "## A\n\n- item 1\n- item 2\n\n## B\n\n- item 3", res.Body)
}

func TestRender_FrontmatterIncludesRuntimeFields(t *testing.T) {
	reg := blocks.NewRegistry()
	tpl := &Template{
		Name: "daily-brief", WriteMode: "marker-block", MarkerID: "x",
		Frontmatter: map[string]any{"type": "daily-brief"},
	}
	r := NewRenderer(reg)
	now := time.Date(2026, 5, 7, 10, 30, 0, 0, time.UTC)
	res, err := r.Render(context.Background(), tpl, blocks.GatherContext{
		Now:     now,
		Sources: map[string]struct{}{"linear": {}, "github": {}},
	})
	require.NoError(t, err)

	assert.Equal(t, "Daily Brief 2026-05-07", res.Frontmatter["title"])
	assert.Equal(t, "2026-05-07", res.Frontmatter["date"])
	assert.Equal(t, "archy", res.Frontmatter["generated_by"])
	assert.NotEmpty(t, res.Frontmatter["generated_at"])
	assert.Equal(t, []string{"github", "linear"}, res.Frontmatter["source_systems"])
	// Template-declared field carried through.
	assert.Equal(t, "daily-brief", res.Frontmatter["type"])
}

func TestRender_FrontmatterTemplateOverridesRuntime(t *testing.T) {
	reg := blocks.NewRegistry()
	tpl := &Template{
		Name: "daily-brief", WriteMode: "marker-block", MarkerID: "x",
		Frontmatter: map[string]any{"title": "Custom Title"},
	}
	r := NewRenderer(reg)
	res, err := r.Render(context.Background(), tpl, blocks.GatherContext{Now: time.Now()})
	require.NoError(t, err)
	assert.Equal(t, "Custom Title", res.Frontmatter["title"])
}

func TestRender_PriorBlockData_LaterBlocksSeeEarlier(t *testing.T) {
	reg := blocks.NewRegistry()
	require.NoError(t, reg.Register(makeBlock("alpha", true, "## A", "alpha-data")))

	// bravo is a scriptedBlock whose Gather inspects gctx.PriorBlockData
	// and embeds the seen value into its rendered output.
	bravo := scriptedBlock{
		name:      "bravo",
		available: true,
		gather: func(gctx blocks.GatherContext) (blocks.BlockData, error) {
			seen, ok := gctx.PriorBlockData["alpha"]
			if !ok {
				return "", errors.New("alpha data missing from PriorBlockData")
			}
			return seen, nil
		},
		render: func(data blocks.BlockData) (string, error) {
			return "## B saw " + data.(string), nil
		},
	}
	require.NoError(t, reg.Register(bravo))

	tpl := &Template{
		Name: "x", WriteMode: "marker-block", MarkerID: "x",
		Blocks: []BlockSpec{{Name: "alpha"}, {Name: "bravo"}},
	}
	r := NewRenderer(reg)
	res, err := r.Render(context.Background(), tpl, blocks.GatherContext{Now: time.Now()})
	require.NoError(t, err)
	assert.True(t, strings.Contains(res.Body, "B saw alpha-data"))
}

func TestRender_UnknownBlockErrorsButContinues(t *testing.T) {
	reg := blocks.NewRegistry()
	require.NoError(t, reg.Register(makeBlock("alpha", true, "## A", nil)))
	tpl := &Template{
		Name: "x", WriteMode: "marker-block", MarkerID: "x",
		Blocks: []BlockSpec{{Name: "missing"}, {Name: "alpha"}},
	}
	r := NewRenderer(reg)
	res, err := r.Render(context.Background(), tpl, blocks.GatherContext{Now: time.Now()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown block "missing"`)
	assert.Equal(t, "## A", res.Body)
}
