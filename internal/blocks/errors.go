package blocks

import "errors"

// ErrDuplicateName is returned by [Registry.Register] when a block
// with the same Name() is already registered.
var ErrDuplicateName = errors.New("blocks: duplicate block name")
