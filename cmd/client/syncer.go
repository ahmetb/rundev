package main

import (
	"github.com/ahmetb/rundev/lib/fsutil"
	"github.com/pkg/errors"
	"io"
)

type syncer struct {
	opts syncOpts
}

func newSyncer(opts syncOpts) *syncer {
	return &syncer{opts: opts}
}

func (s *syncer) checksum() (uint64, error) {
	fs, err := fsutil.Walk(s.opts.localDir)
	if err != nil {
		return 0, errors.Wrap(err, "failed to walk the local fs")
	}
	return fs.Checksum(), nil
}

func (s *syncer) diffFrom(remote fsutil.FSNode) ([]fsutil.DiffOp, error) {
	local, err := fsutil.Walk(s.opts.localDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to walk the local fs")
	}
	return fsutil.FSDiff(local, remote), nil
}

func (s *syncer) prepPatch(ops []fsutil.DiffOp) (io.Reader, error) {
	panic("TODO implement tarballing")
}
