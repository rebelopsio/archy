package render

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/rebelopsio/archy/internal/blocks"
)

// Renderer composes [Template]s into markdown. It looks up each
// declared block in its registry, gathers + renders, and joins the
// outputs.
type Renderer struct {
	// Registry is consulted to resolve each [BlockSpec.Name] to a
	// concrete [blocks.Block]. Registered before Render is called.
	Registry *blocks.Registry
}

// NewRenderer returns a Renderer that draws blocks from registry.
func NewRenderer(registry *blocks.Registry) *Renderer {
	return &Renderer{Registry: registry}
}

// Result is what Render returns.
type Result struct {
	// Body is the composed markdown content (no frontmatter, no
	// marker comments — the writer adds those).
	Body string

	// Frontmatter is a map suitable for write.Note.Frontmatter.
	// Includes template-declared fields plus runtime-computed ones:
	// title, date, generated_at, generated_by ("archy"), and
	// source_systems (sorted).
	Frontmatter map[string]any

	// BlockResults records per-block outcomes for diagnostics, in
	// template order.
	BlockResults []BlockResult
}

// BlockResult is one block's rendering outcome.
type BlockResult struct {
	// Name is the block's identifier from the template.
	Name string
	// Available is the result of the block's Available check.
	Available bool
	// Skipped is true iff Available returned false.
	Skipped bool
	// Output is the markdown the block rendered. Empty on skip or error.
	Output string
	// Error is non-nil if Gather or Render failed for this block.
	// The renderer continues past a block error; the partial body
	// still includes earlier blocks' output.
	Error error
}

// Render walks tpl, gathers and renders each block, and composes the
// final markdown. The returned error is non-nil iff any block errored
// (the partial body and per-block diagnostics are still returned).
func (r *Renderer) Render(ctx context.Context, tpl *Template, gctx blocks.GatherContext) (Result, error) {
	if gctx.PriorBlockData == nil {
		gctx.PriorBlockData = make(map[string]blocks.BlockData)
	}

	var (
		body         strings.Builder
		blockResults = make([]BlockResult, 0, len(tpl.Blocks))
		errs         []error
	)

	for _, spec := range tpl.Blocks {
		block, ok := r.Registry.Get(spec.Name)
		if !ok {
			err := fmt.Errorf("render: unknown block %q", spec.Name)
			errs = append(errs, err)
			blockResults = append(blockResults, BlockResult{Name: spec.Name, Error: err})
			continue
		}

		if !block.Available(ctx, gctx) {
			blockResults = append(blockResults, BlockResult{Name: spec.Name, Available: false, Skipped: true})
			continue
		}

		data, err := block.Gather(ctx, gctx)
		if err != nil {
			wrapped := fmt.Errorf("render: gather %q: %w", spec.Name, err)
			errs = append(errs, wrapped)
			blockResults = append(blockResults, BlockResult{Name: spec.Name, Available: true, Error: wrapped})
			continue
		}
		gctx.PriorBlockData[spec.Name] = data

		out, err := block.Render(ctx, data)
		if err != nil {
			wrapped := fmt.Errorf("render: render %q: %w", spec.Name, err)
			errs = append(errs, wrapped)
			blockResults = append(blockResults, BlockResult{Name: spec.Name, Available: true, Error: wrapped})
			continue
		}

		if out != "" {
			if body.Len() > 0 {
				body.WriteString("\n\n")
			}
			body.WriteString(out)
		}
		blockResults = append(blockResults, BlockResult{Name: spec.Name, Available: true, Output: out})
	}

	return Result{
		Body:         body.String(),
		Frontmatter:  buildFrontmatter(tpl, gctx),
		BlockResults: blockResults,
	}, errors.Join(errs...)
}

// buildFrontmatter merges template-declared frontmatter with
// runtime-computed values. Template fields take precedence — if a
// template explicitly sets "title", the runtime-computed default is
// not added.
func buildFrontmatter(tpl *Template, gctx blocks.GatherContext) map[string]any {
	out := make(map[string]any, len(tpl.Frontmatter)+5)
	for k, v := range tpl.Frontmatter {
		out[k] = v
	}

	now := gctx.Now
	if _, ok := out["title"]; !ok {
		out["title"] = titleFromTemplateName(tpl.Name) + " " + now.Format("2006-01-02")
	}
	if _, ok := out["date"]; !ok {
		out["date"] = now.Format("2006-01-02")
	}
	if _, ok := out["generated_at"]; !ok {
		out["generated_at"] = now.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if _, ok := out["generated_by"]; !ok {
		out["generated_by"] = "archy"
	}
	if len(gctx.Sources) > 0 {
		if _, ok := out["source_systems"]; !ok {
			sources := make([]string, 0, len(gctx.Sources))
			for s := range gctx.Sources {
				sources = append(sources, s)
			}
			sort.Strings(sources)
			out["source_systems"] = sources
		}
	}
	return out
}

// titleFromTemplateName converts "daily-brief" to "Daily Brief".
func titleFromTemplateName(name string) string {
	if name == "" {
		return "Untitled"
	}
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
