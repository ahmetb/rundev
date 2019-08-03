package main

import (
	"encoding/json"
	"fmt"
	"github.com/ahmetb/rundev/lib/constants"
	"github.com/ahmetb/rundev/lib/fsutil"
	"github.com/pkg/errors"
	"io"
	"net/http"
)

type syncOpts struct {
	localDir   string
	targetAddr string
}

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

// uploadPatch creates and uploads a patch to remote endpoint to be applied if it's currently at the given checksum.
func (s *syncer) applyPatch(remoteFS fsutil.FSNode, currentRemoteChecksum string) error {
	localFS, err := fsutil.Walk(s.opts.localDir)
	if err != nil {
		return errors.Wrapf(err, "failed to walk local fs dir %s", s.opts.localDir)
	}
	localChecksum := localFS.Checksum()

	diff := fsutil.FSDiff(localFS, remoteFS)

	tar, err := fsutil.PatchArchive(s.opts.localDir, diff)
	if err != nil {
		return err
	}

	url := s.opts.targetAddr + "/rundevd/patch"
	req, err := http.NewRequest(http.MethodPatch, url, tar)
	if err != nil {
		return errors.Wrap(err, "failed to create patch requeset")
	}
	req.Header.Set("content-type", constants.MimePatch)
	req.Header.Set(constants.HdrRundevPatchPreconditionSum, currentRemoteChecksum)
	req.Header.Set(constants.HdrRundevChecksum, fmt.Sprintf("%d", localChecksum))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "error making patch request")
	}
	resp.Body.Close()
	newRemoteChecksum := resp.Header.Get(constants.HdrRundevChecksum)
	if expected := http.StatusAccepted; resp.StatusCode != expected {
		return errors.Errorf("unexpected patch response status=%d (was expecting http %d) (new remote checksum: %s, old remote checksum: %s, local: %d)",
			resp.StatusCode, expected, newRemoteChecksum, currentRemoteChecksum, localChecksum)
	}
	return nil
}

// parseMismatchResponse decodes checksum mismatch response body which contains remote filesystem root node.
func parseMismatchResponse(body io.ReadCloser) (fsutil.FSNode, error) {
	defer body.Close()
	var v fsutil.FSNode
	d := json.NewDecoder(body)
	d.DisallowUnknownFields()
	err := d.Decode(&v)
	return v, errors.Wrap(err, "failed to decode checksum mismatch response body")
}
