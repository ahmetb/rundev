package main

type diffType int

const (
	diffOpAdd diffType = iota
	diffOpDel
	diffOpMod
)

type diffOp struct {
	diffType
	fsNode
}

func diff(n1, n2 fsNode) []diffOp {
	// TODO(ahmetb) implement
	return nil
}
