package write

import (
	"errors"
	"testing"
)

func TestSentinels_Distinct(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrInvalidMarkerID", ErrInvalidMarkerID},
		{"ErrPathEscape", ErrPathEscape},
		{"ErrDuplicateMarker", ErrDuplicateMarker},
		{"ErrUnclosedMarker", ErrUnclosedMarker},
		{"ErrVaultRootInvalid", ErrVaultRootInvalid},
	}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				continue
			}
			if errors.Is(a.err, b.err) {
				t.Errorf("%s.Is(%s) should be false", a.name, b.name)
			}
		}
	}
}
