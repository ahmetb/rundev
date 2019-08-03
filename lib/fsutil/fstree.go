package fsutil

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

type FSNode struct {
	Name  string      `json:"name"`
	Mode  os.FileMode `json:"mode"`
	Size  int64       `json:"size,omitempty"`
	Mtime time.Time   `json:"mtime"`
	Nodes []FSNode    `json:"nodes,omitempty"`
}

func (f FSNode) String() string {
	return "(" + f.Mode.String() + ") " +
		f.Name + " (" + fmt.Sprintf("%d", len(f.Nodes)) + ") nodes"
}

func (f FSNode) Checksum() uint64 {
	h := fnv.New64()
	h.Write([]byte(f.Name))
	a1 := uint64(f.Size)
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
	n.Name = "$root" // value doesn't matter, but should be consistent on local vs remote
	return n, errors.Wrap(err, "failed to traverse directory tree")
}

func walkFile(path string, fi os.FileInfo) (FSNode, error) {
	n := FSNode{
		Name:  fi.Name(),
		Mode:  fi.Mode(),
		Size:  fi.Size(),
		Mtime: fi.ModTime().Truncate(time.Second), // tarballs don't support nsecs in time spec
	}
	if !fi.IsDir() {
		return n, nil
	}
	n.Size = 0                // zero size for dirs
	n.Mtime = time.Unix(0, 0) // zero time for dirs

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
