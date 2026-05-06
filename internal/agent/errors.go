package agent

import "errors"

// ErrSetup is returned by [New] when its arguments are malformed or the
// SDK cannot be initialized.
var ErrSetup = errors.New("agent setup failed")

// ErrRun is returned by [Runtime.Run] when the SDK reports a protocol
// error or the agent loop fails. Cancellation errors are not wrapped in
// ErrRun — they wrap ctx.Err() directly.
var ErrRun = errors.New("agent run failed")
