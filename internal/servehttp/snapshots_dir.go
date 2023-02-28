package servehttp

import (
	"context"
	"github.com/restic/restic/internal/restic"
	"io/fs"
	"time"
)

// snapshotsDirectory represent the directory with snapshots as a subdirectories.
// Implements fs.FileInfo and has Open() method returning fs.File.
type snapshotsDirectory struct {
	snapshots []*snapshot
	names     map[string]*snapshot

	// TODO: update snapshots once for a while
	//snCount   int
	//lastCheck time.Time
	//mu sync.Mutex
}

func newSnapshotsDirectory(ctx context.Context, repo restic.Repository, root *Root) (*snapshotsDirectory, error) {
	snapshots, err := restic.FindFilteredSnapshots(ctx, repo.Backend(), repo, nil, nil, nil)
	if err != nil {
		return nil, err
	}

	dir := &snapshotsDirectory{
		snapshots: nil,
		names:     map[string]*snapshot{},
	}

	for _, s := range snapshots {
		snap := newSnapshot(s, root)

		dir.snapshots = append(dir.snapshots, snap)
		dir.names[snap.Name()] = snap
	}

	return dir, nil
}

func (s *snapshotsDirectory) Open() fs.File {
	var entries []fs.DirEntry
	for _, snap := range s.snapshots {
		entries = append(entries, fs.FileInfoToDirEntry(snap))
	}
	return newMapDir(s, entries)
}

func (s *snapshotsDirectory) Name() string {
	return ""
}

func (s *snapshotsDirectory) Size() int64 {
	return 0
}

func (s *snapshotsDirectory) Mode() fs.FileMode {
	return fs.ModeDir
}

func (s *snapshotsDirectory) ModTime() time.Time {
	return time.Time{}
}

func (s *snapshotsDirectory) IsDir() bool {
	return true
}

func (s *snapshotsDirectory) Sys() interface{} {
	return nil
}

func (s *snapshotsDirectory) LookupSnapshot(name string) *snapshot {
	return s.names[name]
}
