package fsutil

import (
	"encoding/binary"
	"hash/fnv"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

type FSNode struct {
	Name  string
	Mode  os.FileMode
	Size  int64
	Mtime time.Time
	Nodes []FSNode
}

func (f FSNode) Checksum() uint64 {
	h := fnv.New64()
	h.Write([]byte(f.Name))
	a1 := uint64(f.Size)
	if f.Mode.IsDir() {
		a1 = 0 // zero Size for dirs
	}
	a2 := uint64(f.Mode)
	a3 := uint64(f.Mtime.UnixNano())

	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, a1)
	h.Write(b)
	binary.LittleEndian.PutUint64(b, a2)
	h.Write(b)
	binary.LittleEndian.PutUint64(b, a3)
	h.Write(b)
	for _, c := range f.Nodes {
		v := c.Checksum()
		binary.LittleEndian.PutUint64(b, v)
		h.Write(b)
	}
	return h.Sum64()
}

func Walk(dir string) (FSNode, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return FSNode{}, errors.Wrapf(err, "failed to open directory %s", dir)
	}
	if !fi.IsDir() {
		return FSNode{}, errors.Errorf("path %s is not a directory", dir)
	}

	n, err := walkFile(dir, fi)
	return n, errors.Wrap(err, "failed to traverse directory tree")
}

func walkFile(path string, fi os.FileInfo) (FSNode, error) {
	n := FSNode{
		Name:  fi.Name(),
		Mode:  fi.Mode(),
		Size:  fi.Size(),
		Mtime: fi.ModTime(),
	}
	if !fi.IsDir() {
		return n, nil
	}

	children, err := ioutil.ReadDir(path)
	if err != nil {
		return FSNode{}, errors.Wrapf(err, "failed to list files in directory %s", path)
	}
	if len(children) > 0 {
		n.Nodes = make([]FSNode, 0, len(children))
	}
	for _, f := range children {
		v, err := walkFile(filepath.Join(path, f.Name()), f)
		if err != nil {
			return FSNode{}, err
		}
		n.Nodes = append(n.Nodes, v)
	}
	return n, nil
}
