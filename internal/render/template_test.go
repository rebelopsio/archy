package render

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTemplate(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tpl.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestLoadTemplate_Valid(t *testing.T) {
	path := writeTemplate(t, `
name: daily-brief
write_mode: marker-block
marker_id: daily-brief

frontmatter:
  type: daily-brief
  tags: [daily, work]

blocks:
  - top_priorities:
      sources: [linear]
      limit: 5
  - synthesis:
      depends_on: [top_priorities]
      style: brief
`)
	tpl, err := LoadTemplate(path)
	require.NoError(t, err)
	assert.Equal(t, "daily-brief", tpl.Name)
	assert.Equal(t, "marker-block", tpl.WriteMode)
	assert.Equal(t, "daily-brief", tpl.MarkerID)
	assert.Equal(t, "daily-brief", tpl.Frontmatter["type"])
	require.Len(t, tpl.Blocks, 2)
	assert.Equal(t, "top_priorities", tpl.Blocks[0].Name)
	assert.Equal(t, 5, tpl.Blocks[0].Config["limit"])
	assert.Equal(t, "synthesis", tpl.Blocks[1].Name)
}

func TestLoadTemplate_MalformedYAML(t *testing.T) {
	path := writeTemplate(t, "this: is: not: valid: yaml: at all\n  - [\n")
	_, err := LoadTemplate(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse template")
}

func TestLoadTemplate_MissingName(t *testing.T) {
	path := writeTemplate(t, `
write_mode: marker-block
marker_id: x
blocks: []
`)
	_, err := LoadTemplate(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestLoadTemplate_MissingWriteMode(t *testing.T) {
	path := writeTemplate(t, `
name: daily-brief
marker_id: x
blocks: []
`)
	_, err := LoadTemplate(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write_mode is required")
}

func TestLoadTemplate_MissingMarkerID(t *testing.T) {
	path := writeTemplate(t, `
name: daily-brief
write_mode: marker-block
blocks: []
`)
	_, err := LoadTemplate(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marker_id is required")
}

func TestLoadTemplate_FileMissing(t *testing.T) {
	_, err := LoadTemplate(filepath.Join(t.TempDir(), "no-such-file.yaml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read template")
}

func TestLoadTemplate_BlockEntryWithMultipleKeys(t *testing.T) {
	path := writeTemplate(t, `
name: daily-brief
write_mode: marker-block
marker_id: x
blocks:
  - top_priorities:
      limit: 5
    synthesis:
      style: brief
`)
	_, err := LoadTemplate(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "want 1")
}
