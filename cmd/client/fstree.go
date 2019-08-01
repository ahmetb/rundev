package main

import (
	"encoding/binary"
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
	h.Write([]byte(f.name))
	a1 := uint64(f.size)
	if f.mode.IsDir() {
		a1 = 0 // zero size for dirs
	}
	a2 := uint64(f.mode)
	a3 := uint64(f.mtime.UnixNano())

	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, a1)
	h.Write(b)
	binary.LittleEndian.PutUint64(b, a2)
	h.Write(b)
	binary.LittleEndian.PutUint64(b, a3)
	h.Write(b)
	for _, c := range f.nodes {
		v := c.checksum()
		binary.LittleEndian.PutUint64(b, v)
		h.Write(b)
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
