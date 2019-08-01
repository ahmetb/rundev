package fsutil

import "path/filepath"

type diffType int

const (
	DiffOpAdd diffType = iota
	diffOpDel
)

type DiffOp struct {
	Type diffType
	Path string
}

// FSDiff returns the operations that needs to be done on n2 to make it look like n1
func FSDiff(n1, n2 FSNode) []DiffOp {
	return fsDiffInner(n1, n2, ".")
}

func fsDiffInner(n1, n2 FSNode, base string) []DiffOp {
	var ops []DiffOp
	ln := n1.Nodes
	rn := n2.Nodes
	for len(ln) > 0 && len(rn) > 0 {
		l, r := ln[0], rn[0]

		if l.Name < r.Name { // file doesn't exist in r
			ops = append(ops, DiffOp{DiffOpAdd, canonicalPath(base, l.Name)})
			ln = ln[1:] // advance
		} else if l.Name > r.Name { // file doesn't exist in l
			ops = append(ops, DiffOp{diffOpDel, canonicalPath(base, r.Name)})
			rn = rn[1:]
		} else { // l.Name == r.Name (same item)
			if l.Mode.IsDir() != r.Mode.IsDir() { // one of them is a directory
				ops = append(ops, DiffOp{diffOpDel, canonicalPath(base, l.Name)})
				ops = append(ops, DiffOp{DiffOpAdd, canonicalPath(base, r.Name)})
			} else if l.Checksum() != r.Checksum() { // Checksum mismatch (Size, Mtime, chmod)
				if !l.Mode.IsDir() && !r.Mode.IsDir() {
					// Nodes are not dir, re-upload file
					ops = append(ops, DiffOp{DiffOpAdd, canonicalPath(base, l.Name)})
				} else {
					// both Nodes are dir, recurse:
					ops = append(ops, fsDiffInner(l, r, canonicalPath(base, l.Name))...)
				}
			}
			ln, rn = ln[1:], rn[1:]
		}
	}
	// add remaining
	for _, l := range ln {
		ops = append(ops, DiffOp{DiffOpAdd, canonicalPath(base, l.Name)})
	}
	for _, r := range rn {
		ops = append(ops, DiffOp{diffOpDel, canonicalPath(base, r.Name)})
	}
	return ops
}

// canonicalPath joins base and rel to create a canonical path string with unix path separator (/) independent of
// current platform.
func canonicalPath(base, rel string) string { return filepath.ToSlash(filepath.Join(base, rel)) }
