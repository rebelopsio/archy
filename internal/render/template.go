package render

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Template is the parsed form of a YAML template file. Loaded once
// per workflow run; the same template can be rendered many times
// against different gather contexts.
type Template struct {
	// Name is the template's stable identifier (e.g., "daily-brief").
	Name string
	// WriteMode is one of "marker-block", "overwrite", "append".
	// Consumed by the writer (not the renderer); the renderer just
	// passes it through on Result so the caller can wire it.
	WriteMode string
	// MarkerID is the marker-block id when WriteMode is marker-block.
	MarkerID string
	// Frontmatter is the static frontmatter fields declared by the
	// template. The renderer merges these with runtime-computed
	// fields (title, date, generated_at, generated_by, source_systems)
	// when building [Result.Frontmatter].
	Frontmatter map[string]any
	// Blocks is the ordered list of block invocations. Each spec
	// names a block and carries its per-invocation config.
	Blocks []BlockSpec
}

// BlockSpec is one block's invocation in a template.
type BlockSpec struct {
	// Name is the block identifier the [blocks.Registry] resolves.
	Name string
	// Config is the block's per-invocation configuration. Block
	// implementations interpret these keys; the renderer doesn't.
	Config map[string]any
}

// templateYAML is the on-disk shape of a template file. Each entry in
// blocks: is a single-key map whose key is the block name and value
// is the per-block config. Decoded into the public [Template] type
// after a small unwrap.
type templateYAML struct {
	Name        string                      `yaml:"name"`
	WriteMode   string                      `yaml:"write_mode"`
	MarkerID    string                      `yaml:"marker_id"`
	Frontmatter map[string]any              `yaml:"frontmatter"`
	Blocks      []map[string]map[string]any `yaml:"blocks"`
}

// LoadTemplate reads and parses a template YAML file. Returns a clear
// error wrapped with the path on read or parse failure, and rejects
// templates missing required fields (name, write_mode, marker_id).
func LoadTemplate(path string) (*Template, error) {
	data, err := os.ReadFile(path) //nolint:gosec // template path is operator-supplied; path validation is the caller's job
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", path, err)
	}
	var raw templateYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse template %s: %w", path, err)
	}
	tpl := &Template{
		Name:        raw.Name,
		WriteMode:   raw.WriteMode,
		MarkerID:    raw.MarkerID,
		Frontmatter: raw.Frontmatter,
	}
	for i, entry := range raw.Blocks {
		if len(entry) == 0 {
			return nil, fmt.Errorf("parse template %s: blocks[%d] is empty", path, i)
		}
		if len(entry) > 1 {
			return nil, fmt.Errorf("parse template %s: blocks[%d] has %d keys, want 1", path, i, len(entry))
		}
		for name, cfg := range entry {
			tpl.Blocks = append(tpl.Blocks, BlockSpec{Name: name, Config: cfg})
		}
	}
	if err := validateTemplate(tpl); err != nil {
		return nil, fmt.Errorf("validate template %s: %w", path, err)
	}
	return tpl, nil
}

// validateTemplate enforces the required-fields contract.
func validateTemplate(tpl *Template) error {
	var errs []error
	if tpl.Name == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if tpl.WriteMode == "" {
		errs = append(errs, errors.New("write_mode is required"))
	}
	if tpl.MarkerID == "" {
		errs = append(errs, errors.New("marker_id is required"))
	}
	return errors.Join(errs...)
}
