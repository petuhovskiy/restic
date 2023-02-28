package servehttp

import (
	"context"
	"github.com/restic/restic/internal/bloblru"
	"github.com/restic/restic/internal/debug"
	"github.com/restic/restic/internal/restic"
	"io/fs"
	path2 "path"
)

const blobCacheSize = 64 << 20

type Config struct {
}

// Root is the root node of the HTTP file server, serving a repository.
// It contains list of snapshots.
type Root struct {
	repo      restic.Repository
	cfg       Config
	blobCache *bloblru.Cache

	snapshotsDir *snapshotsDirectory
}

// ensure that *Root implements these interfaces
var _ = fs.FS(&Root{})

// NewRoot initializes a new root node from a repository.
func NewRoot(ctx context.Context, repo restic.Repository, cfg Config) (*Root, error) {
	debug.Log("NewRoot(), config %v", cfg)

	root := &Root{
		repo:         repo,
		cfg:          cfg,
		blobCache:    bloblru.New(blobCacheSize),
		snapshotsDir: nil,
	}

	snapshotsDir, err := newSnapshotsDirectory(ctx, repo, root)
	if err != nil {
		return nil, err
	}
	root.snapshotsDir = snapshotsDir

	err = repo.LoadIndex(ctx)
	if err != nil {
		return nil, err
	}

	return root, nil
}

func (r *Root) Open(name string) (fs.File, error) {
	debug.Log("Open %s", name)

	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	if name == "." {
		return r.snapshotsDir.Open(), nil
	}

	name = path2.Clean(name)
	path := splitPath(name)
	if len(path) == 0 {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	snapshotName := path[0]
	snapshot := r.snapshotsDir.LookupSnapshot(snapshotName)
	if snapshot == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	return snapshot.Lookup(context.Background(), path[1:], "", nil)
}

func splitPath(p string) []string {
	d, f := path2.Split(p)
	if d == "" || d == "/" {
		return []string{f}
	}
	s := splitPath(path2.Join("/", d))
	return append(s, f)
}
