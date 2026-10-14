// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dagucloud/dagu/v2/internal/cmn/fileutil"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger/tag"
)

// changeMarkerSuffix ends the name of every change marker file.
const changeMarkerSuffix = ".changed"

// staleMarkerAge is how old a marker file must be before readers ignore it.
// It lets files left behind by decommissioned hosts or pods be disregarded
// without ever deleting them.
const staleMarkerAge = 7 * 24 * time.Hour

// unknownHostID names the marker file of a host whose name could not be read.
const unknownHostID = "unknown"

// localHostName is the name of this host, or "" when it is unknown.
var localHostName = sync.OnceValue(func() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(host)
})

// ChangeMarkerPath returns the marker file this host writes to signal DAG
// definition changes for baseDir. The marker lives in indexDir, outside the DAG
// directory. Every host that uses the same data directory has its own file,
// "<base hash>.<host hash>.changed", so a writer never overwrites another
// host's marker. It returns "" when there is no index directory, which
// disables the marker.
func ChangeMarkerPath(indexDir, baseDir string) string {
	return changeMarkerPathForHost(indexDir, baseDir, localHostName())
}

func changeMarkerPathForHost(indexDir, baseDir, host string) string {
	if indexDir == "" {
		return ""
	}
	if abs, err := filepath.Abs(baseDir); err == nil {
		baseDir = abs
	}
	sum := sha256.Sum256([]byte(filepath.Clean(baseDir)))
	return filepath.Join(indexDir, hex.EncodeToString(sum[:8])+"."+hostID(host)+changeMarkerSuffix)
}

// hostID names a host in a marker file name.
func hostID(host string) string {
	if host == "" {
		return unknownHostID
	}
	sum := sha256.Sum256([]byte(host))
	return hex.EncodeToString(sum[:8])
}

// ChangeMarker is a tiny file that is rewritten whenever a DAG definition is
// created, updated, deleted or renamed through the Dagu store (the UI and API
// editors, and base configuration edits). File watches do not fire for writes
// that another host makes on a shared network volume, so a reader polls the
// markers of the other hosts instead. The marker is best effort: it is a hint
// that something may have changed, never the source of truth.
//
// Files that Dagu does not write through the store are not signalled: DAG
// files edited directly on the volume and changes pulled by git sync are only
// picked up by the reader's periodic resync. The marker name derives from the
// DAG directory path, so every host must mount that directory at the same
// path; otherwise only the resync applies.
//
// Each host writes only its own file and a reader ignores its own host's file,
// because its file watches already report those changes. Ignoring it cannot
// hide a bump from another host, which lives in a different file. A host whose
// name is unknown, and two machines that share a host name, share one file and
// cannot tell their bumps apart: an unknown host therefore ignores nothing,
// and machines sharing a name rely on the resync.
type ChangeMarker struct {
	path string
}

// NewChangeMarker returns a marker backed by path, as returned by
// ChangeMarkerPath. An empty path disables it.
func NewChangeMarker(path string) *ChangeMarker {
	return &ChangeMarker{path: path}
}

// Bump records that DAG definitions changed. Failures are logged and ignored.
func (m *ChangeMarker) Bump(ctx context.Context) {
	if m == nil || m.path == "" {
		return
	}
	token, err := newChangeToken()
	if err != nil {
		logger.Warn(ctx, "Failed to create DAG change marker value", tag.Error(err))
		return
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0750); err != nil {
		logger.Warn(ctx, "Failed to update DAG change marker", tag.File(m.path), tag.Error(err))
		return
	}
	if err := fileutil.WriteFileAtomic(m.path, []byte(token), defaultPerm); err != nil {
		logger.Warn(ctx, "Failed to update DAG change marker", tag.File(m.path), tag.Error(err))
	}
}

// markerVersions maps marker file names to the file info seen last.
type markerVersions map[string]fs.FileInfo

// poll lists the marker directory and returns the versions to remember and
// whether the marker of any other host is new or changed since last. This
// host's own marker is recorded but never counts, unless the host is unknown.
// Removed markers are dropped without counting, markers not modified for
// staleMarkerAge are ignored, and an unreadable directory counts as unchanged.
//
// Only the directory is read, never the marker files. On network file systems
// open and stat answer from attribute and directory entry caches that can be
// tens of seconds old (up to acregmax, 60s by default on NFS), so a remote
// update would be seen late. Listing the directory revalidates it and, through
// READDIRPLUS, refreshes the attributes of its entries. This relies on the NFS
// client's directory read refreshing attributes, which is the default on Linux
// clients; with the nordirplus mount option detection is bounded by the
// attribute cache timeouts instead. The directory is expected to hold only a
// few small files.
func (m *ChangeMarker) poll(ctx context.Context, last markerVersions) (markerVersions, bool) {
	if m == nil || m.path == "" {
		return last, false
	}
	entries, err := os.ReadDir(filepath.Dir(m.path))
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Debug(ctx, "Failed to list DAG change marker directory", tag.File(m.path), tag.Error(err))
		}
		return last, false
	}

	own := filepath.Base(m.path)
	prefix, _, _ := strings.Cut(own, ".")
	prefix += "."
	ignoreOwn := !strings.HasSuffix(own, "."+unknownHostID+changeMarkerSuffix)

	next := make(markerVersions)
	changed := false
	for _, entry := range entries {
		name := entry.Name()
		if len(name) <= len(prefix)+len(changeMarkerSuffix) ||
			!strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, changeMarkerSuffix) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || time.Since(info.ModTime()) > staleMarkerAge {
			continue
		}
		next[name] = info
		if name == own && ignoreOwn {
			continue
		}
		if prev, ok := last[name]; !ok || !sameMarkerVersion(prev, info) {
			changed = true
		}
	}
	return next, changed
}

// sameMarkerVersion reports whether two infos describe the same version of a
// marker file: an atomic rewrite replaces the file, so its identity or time
// changes.
func sameMarkerVersion(a, b fs.FileInfo) bool {
	return os.SameFile(a, b) && a.ModTime().Equal(b.ModTime()) && a.Size() == b.Size()
}

func newChangeToken() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(random[:])), nil
}
