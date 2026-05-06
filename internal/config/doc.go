// Package config loads, validates, and exposes archy's runtime configuration.
//
// Linux and macOS only. The default config location is the path returned by
// os.UserConfigDir joined with "archy/config.yaml" — that is,
// $XDG_CONFIG_HOME/archy/config.yaml on Linux (or ~/.config/archy/config.yaml
// when XDG_CONFIG_HOME is unset) and ~/Library/Application Support/archy/config.yaml
// on macOS.
//
// Precedence, lowest to highest: package defaults → file values →
// ARCHY_-prefixed environment variables.
//
// Per ADR-0002 this package is a leaf: it imports nothing else from
// internal/.
package config
