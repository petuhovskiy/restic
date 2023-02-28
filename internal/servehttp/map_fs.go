package servehttp

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"time"
)

// A mapDir is a directory fs.File (so also an fs.ReadDirFile) open for reading.
type mapDir struct {
	fileInfo fs.FileInfo
	entry    []fs.DirEntry
	offset   int
}

func newMapDir(fileInfo fs.FileInfo, entry []fs.DirEntry) *mapDir {
	return &mapDir{
		fileInfo: fileInfo,
		entry:    entry,
	}
}

func (d *mapDir) Stat() (fs.FileInfo, error) { return d.fileInfo, nil }
func (d *mapDir) Close() error               { return nil }
func (d *mapDir) Read(b []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.fileInfo.Name(), Err: errors.New("is a directory")}
}

func (d *mapDir) ReadDir(count int) ([]fs.DirEntry, error) {
	n := len(d.entry) - d.offset
	if n == 0 && count > 0 {
		return nil, io.EOF
	}
	if count > 0 && n > count {
		n = count
	}
	list := make([]fs.DirEntry, n)
	for i := range list {
		list[i] = d.entry[d.offset+i]
	}
	d.offset += n
	return list, nil
}

// mapFI is the map-based implementation of FileInfo.
type mapFI struct {
	name string
	size int
	dir  bool
}

func (fi mapFI) IsDir() bool        { return fi.dir }
func (fi mapFI) ModTime() time.Time { return time.Time{} }
func (fi mapFI) Mode() os.FileMode {
	if fi.IsDir() {
		return 0755 | os.ModeDir
	}
	return 0444
}
func (fi mapFI) Name() string     { return fi.name }
func (fi mapFI) Size() int64      { return int64(fi.size) }
func (fi mapFI) Sys() interface{} { return nil }
