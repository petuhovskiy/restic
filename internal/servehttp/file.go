package servehttp

import (
	"context"
	"fmt"
	"github.com/restic/restic/internal/debug"
	"github.com/restic/restic/internal/errors"
	"github.com/restic/restic/internal/restic"
	"io"
	"io/fs"
	"os"
	"sort"
)

type file struct {
	root    *Root
	ctx     context.Context
	node    *restic.Node
	cumsize []uint64
	offset  int
}

func openFile(ctx context.Context, root *Root, node *restic.Node) (*file, error) {
	f := &file{
		root: root,
		ctx:  ctx,
		node: node,
	}
	err := f.open()
	return f, err
}

func (f *file) Stat() (fs.FileInfo, error) {
	return &mapFI{
		name: f.node.Name,
		size: int(f.node.Size),
		dir:  false,
	}, nil
}

func (f *file) Read(bytes []byte) (int, error) {
	res, err := f.read(f.ctx, uint64(f.offset), uint64(len(bytes)), bytes)
	f.offset += res
	return res, err
}

func (f *file) Close() error {
	return nil
}

func (f *file) ReadAt(p []byte, off int64) (n int, err error) {
	return f.read(f.ctx, uint64(off), uint64(len(p)), p)
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += int64(f.offset)
	case io.SeekEnd:
		fi, err := f.Stat()
		if err != nil {
			return int64(f.offset), err
		}
		offset += fi.Size()
	default:
		return int64(f.offset), errors.New("not implemented")
	}

	if offset < 0 {
		return int64(f.offset), os.ErrInvalid
	}

	f.offset = int(offset)
	return int64(f.offset), nil
}

func (f *file) open() error {
	debug.Log("open file %v with %d blobs", f.node.Name, len(f.node.Content))

	var bytes uint64
	cumsize := make([]uint64, 1+len(f.node.Content))
	for i, id := range f.node.Content {
		size, found := f.root.repo.LookupBlobSize(id, restic.DataBlob)
		if !found {
			return fmt.Errorf("id %v not found in repository", id)
		}

		bytes += uint64(size)
		cumsize[i+1] = bytes
	}

	if bytes != f.node.Size {
		debug.Log("sizes do not match: node.Size %v != size %v, using real size", f.node.Size, bytes)
		// Make a copy of the node with correct size
		nodenew := *f.node
		nodenew.Size = bytes
		f.node = &nodenew
	}
	f.cumsize = cumsize

	return nil
}

func (f *file) getBlobAt(ctx context.Context, i int) (blob []byte, err error) {
	blob, ok := f.root.blobCache.Get(f.node.Content[i])
	if ok {
		return blob, nil
	}

	blob, err = f.root.repo.LoadBlob(ctx, restic.DataBlob, f.node.Content[i], nil)
	if err != nil {
		debug.Log("LoadBlob(%v, %v) failed: %v", f.node.Name, f.node.Content[i], err)
		return nil, err
	}

	f.root.blobCache.Add(f.node.Content[i], blob)

	return blob, nil
}

func (f *file) read(ctx context.Context, offset, size uint64, data []byte) (int, error) {
	// Skip blobs before the offset
	startContent := -1 + sort.Search(len(f.cumsize), func(i int) bool {
		return f.cumsize[i] > offset
	})
	offset -= f.cumsize[startContent]

	dst := data[0:size]
	readBytes := 0
	remainingBytes := size

	for i := startContent; remainingBytes > 0 && i < len(f.cumsize)-1; i++ {
		blob, err := f.getBlobAt(ctx, i)
		if err != nil {
			return readBytes, err
		}

		if offset > 0 {
			blob = blob[offset:]
			offset = 0
		}

		copied := copy(dst, blob)
		remainingBytes -= uint64(copied)
		readBytes += copied

		dst = dst[copied:]
	}
	data = data[:readBytes]

	if readBytes == 0 {
		return 0, io.EOF
	}

	return readBytes, nil
}
