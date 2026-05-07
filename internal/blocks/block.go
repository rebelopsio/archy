package blocks

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rebelopsio/archy/internal/domain"
)

// Block is one composable section of a generated note. Concrete
// blocks declare their identity, decide whether they have the data
// they need, gather that data, and render markdown.
type Block interface {
	// Name is the stable identifier templates use to reference this
	// block. Lowercase, snake_case if multi-word.
	Name() string

	// Available reports whether this block has the data sources it
	// needs in the given context. A block whose Available returns
	// false is silently skipped by the renderer.
	Available(ctx context.Context, gctx GatherContext) bool

	// Gather collects the data this block needs. Pure on input —
	// blocks should not mutate gctx or take external locks.
	Gather(ctx context.Context, gctx GatherContext) (BlockData, error)

	// Render produces the markdown for this block. Called after Gather
	// with the value Gather returned. Must be deterministic for data
	// blocks. Synthesis-style blocks may use an LLM in production via
	// the synthesizer bridge described in ADR-0004; v1 uses a
	// deterministic placeholder so the test suite stays free of LLM
	// dependencies.
	Render(ctx context.Context, data BlockData) (string, error)
}

// GatherContext provides the inputs blocks need from the wider system.
// Each block reads what it needs and ignores the rest. Constructed
// once per render by the workflow runner; not modified during rendering.
type GatherContext struct {
	// Now is the reference time for time-dependent blocks.
	Now time.Time

	// Sources is the set of providers configured for this run, by
	// name (e.g., "linear", "github"). Blocks consult this to decide
	// whether they can run via [Block.Available].
	Sources map[string]struct{}

	// Issues, PullRequests, and CalendarEvents are the data the
	// orchestrator has gathered before rendering. v1 populates only
	// Issues; the other two fields are present so future blocks see
	// them without a struct change.
	Issues         []domain.Issue
	PullRequests   []domain.PullRequest
	CalendarEvents []domain.CalendarEvent

	// Scorer ranks items. Blocks call this rather than importing
	// internal/scoring directly, keeping the dependency graph clean.
	Scorer ItemScorer

	// PriorBlockData is the map of completed Gather results, keyed by
	// block name. The renderer populates this incrementally so a
	// later block (e.g., synthesis) can see an earlier block's data
	// (e.g., top_priorities).
	PriorBlockData map[string]BlockData
}

// ItemScorer is the subset of internal/scoring exposed to blocks.
// Concrete implementations live in the workflow runner; tests inject
// fakes.
type ItemScorer interface {
	// ScoreIssues ranks issues by computed priority. The returned
	// slice is in descending score order; ties preserve input order.
	ScoreIssues(ctx context.Context, issues []domain.Issue) []domain.PriorityScore
}

// BlockData is opaque per-block typed data. Each concrete block
// defines its own type and asserts on it in Render. The renderer
// passes the value returned by Gather straight into Render without
// inspecting it.
type BlockData any

// Registry holds a set of registered blocks, keyed by Name. Used by
// the renderer to look up a block's implementation when it appears
// in a template.
type Registry struct {
	mu     sync.RWMutex
	blocks map[string]Block
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{blocks: make(map[string]Block)}
}

// Register adds a block. Returns [ErrDuplicateName] (wrapped) if a
// block with the same Name() is already registered.
func (r *Registry) Register(b Block) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := b.Name()
	if _, exists := r.blocks[name]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateName, name)
	}
	r.blocks[name] = b
	return nil
}

// Get returns a registered block by name. Second return is false if
// no block matches.
func (r *Registry) Get(name string) (Block, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.blocks[name]
	return b, ok
}

// Names returns the registered block names in lexical order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.blocks))
	for n := range r.blocks {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
