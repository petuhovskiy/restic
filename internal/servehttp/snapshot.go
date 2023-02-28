package servehttp

import (
	"context"
	"errors"
	"github.com/restic/restic/internal/restic"
	"io/fs"
	"os"
	"sync"
	"time"
)

type snapshot struct {
	snap *restic.Snapshot
	repo restic.Repository
	root *Root

	trees map[string]*restic.Tree
	mu    sync.Mutex
}

func newSnapshot(snap *restic.Snapshot, root *Root) *snapshot {
	return &snapshot{
		snap:  snap,
		root:  root,
		repo:  root.repo,
		trees: map[string]*restic.Tree{},
	}
}

// fs.FileInfo implementation

func (s *snapshot) Name() string {
	return s.snap.Time.Format(time.RFC3339)
}

func (s *snapshot) IsDir() bool {
	return true
}

func (s *snapshot) Size() int64 {
	return 0
}

func (s *snapshot) Mode() fs.FileMode {
	return fs.ModeDir
}

func (s *snapshot) ModTime() time.Time {
	return time.Time{}
}

func (s *snapshot) Sys() interface{} {
	return nil
}

// Lookup implementation

func (s *snapshot) loadTree(ctx context.Context, path string, id restic.ID) (*restic.Tree, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tree, ok := s.trees[path]
	if ok {
		return tree, nil
	}

	var err error
	tree, err = s.repo.LoadTree(ctx, id)
	if err != nil {
		return nil, err
	}

	s.trees[path] = tree
	return tree, nil
}

func (s *snapshot) Lookup(ctx context.Context, path []string, current string, id *restic.ID) (fs.File, error) {
	if id == nil {
		id = s.snap.Tree
	}

	tree, err := s.loadTree(ctx, current, *id)
	if err != nil {
		return nil, err
	}

	// If we print / we need to assume that there are multiple nodes at that
	// level in the tree.
	if len(path) == 0 {
		var entries []fs.DirEntry
		for _, node := range tree.Nodes {
			fi := nodeToFileInfo(node)
			entries = append(entries, fs.FileInfoToDirEntry(fi))
		}

		return newMapDir(s, entries), nil
	}

	node := tree.Find(path[0])
	if node == nil {
		return nil, errors.New("not found")
	}

	if node.Mode.IsDir() {
		return s.Lookup(ctx, path[1:], current+"/"+path[0], node.Subtree)
	}

	if !node.Mode.IsRegular() {
		return nil, errors.New("not a regular file")
	}

	return openFile(ctx, s.root, node)
}

func nodeToFileInfo(node *restic.Node) os.FileInfo {
	return &mapFI{
		name: node.Name,
		size: int(node.Size),
		dir:  node.Mode.IsDir(),
	}
}
