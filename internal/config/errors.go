package config

import "errors"

// ErrInvalidConfig is returned when a config value fails validation. The
// wrapping error message includes the specific field and reason.
var ErrInvalidConfig = errors.New("invalid config")

// ErrConfigNotFound is returned by Load when the file does not exist.
// LoadDefault does not return this — it falls back to defaults instead.
var ErrConfigNotFound = errors.New("config file not found")

// ErrConfigParse is returned when the YAML cannot be parsed.
var ErrConfigParse = errors.New("config parse error")
