package voice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgress_EnabledAddsEllipsis(t *testing.T) {
	v := Voice{Enabled: true}
	assert.Equal(t, "looking through your linear issues...", v.Progress("looking through your linear issues"))
}

func TestProgress_DisabledReturnsEmpty(t *testing.T) {
	v := Voice{Enabled: false}
	assert.Equal(t, "", v.Progress("anything"))
}

func TestSign_EnabledReturnsSignatureBlock(t *testing.T) {
	v := Voice{Signature: true}
	assert.Equal(t, "\n\n— archy", v.Sign())
}

func TestSign_DisabledReturnsEmpty(t *testing.T) {
	v := Voice{Signature: false}
	assert.Equal(t, "", v.Sign())
}

func TestVoice_ProgressAndSignAreIndependent(t *testing.T) {
	// Progress on, signature off (CLI workflow with quiet output).
	v := Voice{Enabled: true, Signature: false}
	assert.Equal(t, "writing today's brief...", v.Progress("writing today's brief"))
	assert.Equal(t, "", v.Sign())

	// Progress off, signature on (scheduled run with no human watching).
	v = Voice{Enabled: false, Signature: true}
	assert.Equal(t, "", v.Progress("hello"))
	assert.Equal(t, "\n\n— archy", v.Sign())
}
