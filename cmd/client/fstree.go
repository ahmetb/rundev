package main

import (
	"hash/fnv"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

type fsNode struct {
	name  string
	mode  os.FileMode
	size  int64
	mtime time.Time

	nodes []fsNode
}

func (f fsNode) checksum() uint64 {
	h := fnv.New64()
	a1 := f.size
	a2 := int64(f.mode)
	a3 := f.mtime.UnixNano()

	// TODO(ahmetb) can be optimized by pulling all numbers into a single reused slice, or using unsafe
	h.Write([]byte(f.name))
	h.Write([]byte{byte(0xff & a1), byte(0xff00 & a1), byte(0xff0000 & a1), byte(0xff000000 & a1)})
	h.Write([]byte{byte(0xff & a2), byte(0xff00 & a2), byte(0xff0000 & a2), byte(0xff000000 & a2)})
	h.Write([]byte{byte(0xff & a3), byte(0xff00 & a3), byte(0xff0000 & a3), byte(0xff000000 & a3)})
	for _, c := range f.nodes {
		v := c.checksum()
		h.Write([]byte{byte(0xff & v), byte(0xff00 & v), byte(0xff0000 & v), byte(0xff000000 & v)})
	}
	return h.Sum64()
}

func walk(dir string) (fsNode, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return fsNode{}, errors.Wrapf(err, "failed to open directory %s", dir)
	}
	if !fi.IsDir() {
		return fsNode{}, errors.Errorf("path %s is not a directory", dir)
	}

	n, err := walkFile(dir, fi)
	return n, errors.Wrap(err, "failed to traverse directory tree")
}

func walkFile(path string, fi os.FileInfo) (fsNode, error) {
	n := fsNode{
		name:  fi.Name(),
		mode:  fi.Mode(),
		size:  fi.Size(),
		mtime: fi.ModTime(),
	}
	if !fi.IsDir() {
		return n, nil
	}

	children, err := ioutil.ReadDir(path)
	if err != nil {
		return fsNode{}, errors.Wrapf(err, "failed to list files in directory %s", path)
	}
	if len(children) > 0 {
		n.nodes = make([]fsNode, 0, len(children))
	}
	for _, f := range children {
		v, err := walkFile(filepath.Join(path, f.Name()), f)
		if err != nil {
			return fsNode{}, err
		}
		n.nodes = append(n.nodes, v)
	}
	return n, nil
}
