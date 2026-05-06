package write

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// atomicWrite writes data to target via a temp file in the same directory,
// fsyncs the temp file, closes it, then renames it onto target. The rename
// is atomic on POSIX filesystems when source and destination are on the
// same filesystem, which is guaranteed by placing the temp file in the
// target's directory.
//
// Temp file naming: ".<basename>.archy-tmp-<random>". The leading dot
// keeps editors from picking it up; the random suffix avoids collisions
// when two writers race for the same target.
//
// On any failure during create/write/fsync/close/rename, the temp file is
// removed before atomicWrite returns the error.
//
// Permission mode of the resulting file is 0644.
func atomicWrite(target string, data []byte) error {
	dir := filepath.Dir(target)
	base := filepath.Base(target)

	suffix, err := randomSuffix()
	if err != nil {
		return fmt.Errorf("atomic write %s: random suffix: %w", target, err)
	}
	tmpName := "." + base + ".archy-tmp-" + suffix
	tmpPath := filepath.Join(dir, tmpName)

	f, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("atomic write %s: create temp: %w", target, err)
	}
	cleanup := func() {
		_ = os.Remove(tmpPath)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("atomic write %s: write temp: %w", target, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("atomic write %s: fsync: %w", target, err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("atomic write %s: close temp: %w", target, err)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		cleanup()
		return fmt.Errorf("atomic write %s: rename: %w", target, err)
	}
	return nil
}

// randomSuffix returns 16 hex characters of cryptographically random bytes.
// 8 bytes is enough to make accidental collisions effectively impossible
// for the local-process scale of two-or-three concurrent writers archy
// might generate.
func randomSuffix() (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
