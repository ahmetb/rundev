package main

import "path/filepath"

type diffType int

const (
	diffOpAdd diffType = iota
	diffOpDel
)

type diffOp struct {
	Type diffType
	Path string
}

// fsDiff returns the operations that needs to be done on n2 to make it look like n1
func fsDiff(n1, n2 fsNode) []diffOp {
	return fsDiffInner(n1, n2, ".")
}

func fsDiffInner(n1, n2 fsNode, base string) []diffOp {
	var ops []diffOp
	ln := n1.nodes
	rn := n2.nodes
	for len(ln) > 0 && len(rn) > 0 {
		l := ln[0]
		r := rn[0]

		if l.name < r.name { // file doesn't exist in r
			ops = append(ops, diffOp{diffOpAdd, filepath.Join(base, l.name)})
			ln = ln[1:] // advance
		} else if l.name > r.name { // file doesn't exist in l
			ops = append(ops, diffOp{diffOpDel, filepath.Join(base, r.name)})
			rn = rn[1:]
		} else { // l.name == r.name (same item)
			if l.mode.IsDir() != r.mode.IsDir() { // one of them is a directory
				ops = append(ops, diffOp{diffOpDel, filepath.Join(base, l.name)})
				ops = append(ops, diffOp{diffOpAdd, filepath.Join(base, r.name)})
			} else if l.checksum() != r.checksum() { // checksum mismatch (size, mtime, chmod)
				if !l.mode.IsDir() && !r.mode.IsDir() {
					// nodes are not dir, re-upload file
					ops = append(ops, diffOp{diffOpAdd, filepath.Join(base, l.name)})
				} else {
					// both nodes are dir, recurse:
					ops = append(ops, fsDiffInner(l, r, filepath.Join(base, l.name))...)
				}
			}
			ln, rn = ln[1:], rn[1:]
		}
	}
	// add remaining
	for _, l := range ln {
		ops = append(ops, diffOp{diffOpAdd, filepath.Join(base, l.name)})
	}
	for _, r := range rn {
		ops = append(ops, diffOp{diffOpDel, filepath.Join(base, r.name)})
	}
	return ops
}
