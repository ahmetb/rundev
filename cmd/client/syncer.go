package main

import (
	"encoding/json"
	"fmt"
	"github.com/ahmetb/rundev/lib/constants"
	"github.com/ahmetb/rundev/lib/fsutil"
	"github.com/ahmetb/rundev/lib/ignore"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

type syncOpts struct {
	localDir     string
	targetAddr   string
	clientSecret string
	ignores      *ignore.FileIgnores
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
	return fs.RootChecksum(), nil
}

// uploadPatch creates and uploads a patch to remote endpoint to be
// applied if it's currently at the given checksum.
func (s *syncer) uploadPatch(remoteFS fsutil.FSNode, currentRemoteChecksum string) error {
	localFS, err := fsutil.Walk(s.opts.localDir)
	if err != nil {
		return errors.Wrapf(err, "failed to walk local fs dir %s", s.opts.localDir)
	}
	localChecksum := localFS.RootChecksum()

	log.Printf("checksum mismatch local=%d remote=%s", localChecksum, currentRemoteChecksum)
	diff := fsutil.FSDiff(localFS, remoteFS)
	log.Printf("diff operations (%d)", len(diff))
	for _, v := range diff {
		log.Printf("  %s", v)
	}

	tar, n, err := fsutil.PatchArchive(s.opts.localDir, diff)
	if err != nil {
		return err
	}
	log.Printf("diff tarball %d bytes", n)

	url := s.opts.targetAddr + "/rundevd/patch"
	req, err := http.NewRequest(http.MethodPatch, url, tar)
	if err != nil {
		return errors.Wrap(err, "failed to create patch requeset")
	}
	req.Header.Set("Content-Type", constants.MimePatch)
	req.Header.Set(constants.HdrRundevClientSecret, s.opts.clientSecret)
	req.Header.Set(constants.HdrRundevPatchPreconditionSum, currentRemoteChecksum)
	req.Header.Set(constants.HdrRundevChecksum, fmt.Sprintf("%d", localChecksum))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "error making patch request")
	}
	defer resp.Body.Close()
	newRemoteChecksum := resp.Header.Get(constants.HdrRundevChecksum)
	if expected := http.StatusAccepted; resp.StatusCode != expected {
		b, _ := ioutil.ReadAll(resp.Body)
		return errors.Errorf("unexpected patch response status=%d (was expecting http %d) (new remote checksum: %s, old remote checksum: %s, local: %d). response body: %s",
			resp.StatusCode, expected, newRemoteChecksum, currentRemoteChecksum, localChecksum, string(b))
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
