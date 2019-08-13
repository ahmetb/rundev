package fsutil

import (
	"encoding/binary"
	"fmt"
	"github.com/ahmetb/rundev/lib/ignore"
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
	Size  int64       `json:"size,omitempty"` // zero for dirs and whiteout files
	Mtime time.Time   `json:"mtime"`          // in UTC, zero time for dirs
	Nodes []FSNode    `json:"nodes,omitempty"`
}

func (f FSNode) String() string {
	return "(" + f.Mode.String() + ") " +
		f.Name + " (" + fmt.Sprintf("%d", len(f.Nodes)) + ") nodes"
}

// RootChecksum computes the checksum of the directory through its child nodes.
// It doesn't take f’s own name, mode, size and mtime into account.
func (f FSNode) RootChecksum() uint64 {
	return f.childrenChecksum()
}

// checksum computes the checksum of f based on f itself and its children.
func (f FSNode) checksum() uint64 {
	h := fnv.New64()
	h.Write([]byte(f.Name))
	a1 := uint64(f.Size)
	a2 := uint64(f.Mode)
	a3 := uint64(f.Mtime.UnixNano())
	a4 := f.childrenChecksum()

	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, a1)
	h.Write(b)
	binary.LittleEndian.PutUint64(b, a2)
	h.Write(b)
	binary.LittleEndian.PutUint64(b, a3)
	h.Write(b)
	binary.LittleEndian.PutUint64(b, a4)
	h.Write(b)
	return h.Sum64()
}

// childrenChecksum computes the checksum f’s child nodes.
func (f FSNode) childrenChecksum() uint64 {
	h := fnv.New64()
	b := make([]byte, 8)
	for _, c := range f.Nodes {
		v := c.checksum()
		binary.LittleEndian.PutUint64(b, v)
		h.Write(b)
	}
	return h.Sum64()
}

func Walk(dir string, rules *ignore.FileIgnores) (FSNode, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return FSNode{}, errors.Wrapf(err, "failed to open directory %s", dir)
	}
	if !fi.IsDir() {
		return FSNode{}, errors.Errorf("path %s is not a directory", dir)
	}

	n, err := walkFile(dir, dir, fi, rules)
	n.Name = "$root" // value doesn't matter, but should be the same on local vs remote as we don't care about dir basename
	return n, errors.Wrap(err, "failed to traverse directory tree")
}

func walkFile(root, path string, fi os.FileInfo, rules *ignore.FileIgnores) (FSNode, error) {
	n := FSNode{
		Name:  fi.Name(),
		Mode:  fi.Mode(),
		Size:  fi.Size(),
		Mtime: fi.ModTime().Truncate(time.Second).UTC(), // tarballs don't support nsecs in time spec
	}
	if !fi.IsDir() {
		return n, nil
	}
	n.Size = 0                      // zero size for dirs
	n.Mtime = time.Unix(0, 0).UTC() // zero time for dirs

	children, err := ioutil.ReadDir(path)
	if err != nil {
		return FSNode{}, errors.Wrapf(err, "failed to list files in directory %s", path)
	}
	if len(children) > 0 {
		n.Nodes = make([]FSNode, 0, len(children))
	}
	for _, f := range children {
		childPath := filepath.Join(path, f.Name())
		rel, _ := filepath.Rel(root, childPath)
		if rules.Ignored(rel) {
			continue
		}
		v, err := walkFile(root, childPath, f, rules)
		if err != nil {
			return FSNode{}, err
		}
		n.Nodes = append(n.Nodes, v)
	}
	return n, nil
}
