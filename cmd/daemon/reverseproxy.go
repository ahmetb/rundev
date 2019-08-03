package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ahmetb/rundev/lib/constants"
	"github.com/ahmetb/rundev/lib/fsutil"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

type reverseProxier struct {
	syncOpts syncOpts
	next     http.RoundTripper
}

type syncOpts struct {
	syncDir string
}

func reverseProxyTransport(next http.RoundTripper, syncOpts syncOpts) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &reverseProxier{
		syncOpts: syncOpts,
		next:     next,
	}
}

func (rp reverseProxier) RoundTrip(req *http.Request) (*http.Response, error) {
	reqChecksumHdr := req.Header.Get(constants.HdrRundevChecksum)
	if reqChecksumHdr == "" {
		return mkErrorResp(http.StatusBadRequest, errors.Errorf("missing %s header from the client", constants.HdrRundevChecksum)), nil
	}
	reqChecksum, err := strconv.ParseUint(reqChecksumHdr, 10, 64)
	if reqChecksumHdr == "" {
		return mkErrorResp(http.StatusBadRequest, errors.Wrapf(err, "malformed %s", constants.HdrRundevChecksum)), nil
	}

	fs, err := fsutil.Walk(rp.syncOpts.syncDir)
	if err != nil {
		return mkErrorResp(http.StatusInternalServerError, errors.Wrap(err, "failed to walk the sync directory")), nil
	}
	respChecksum := fs.Checksum()

	if respChecksum != reqChecksum {
		return withChecksumHeader(mkChecksumMismatchResp(fs), respChecksum), nil
	}

	resp, err := rp.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	return withChecksumHeader(resp, respChecksum), nil
}

func withChecksumHeader(resp *http.Response, checksum uint64) *http.Response {
	resp.Header.Set(constants.HdrRundevChecksum, fmt.Sprintf("%d", checksum))
	return resp
}

// newReverseProxy returns a reverse proxy to the user’s app.
func newReverseProxy(target *url.URL, syncOpts syncOpts) http.Handler {
	rp := httputil.NewSingleHostReverseProxy(target)
	rp.Transport = reverseProxyTransport(rp.Transport, syncOpts)
	return rp
}

func mkErrorResp(code int, err error) *http.Response {
	return &http.Response{StatusCode: code,
		Body: mkErrorBody(err)}
}

func mkErrorBody(err error) io.ReadCloser { return ioutil.NopCloser(strings.NewReader(err.Error())) }

func mkChecksumMismatchResp(fs fsutil.FSNode) *http.Response {
	hdr := http.Header{}
	hdr.Set(constants.HdrRundevChecksum, fmt.Sprintf("%d", fs.Checksum()))
	hdr.Set("Content-Type", constants.MimeChecksumMismatch)

	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(fs); err != nil {
		return mkErrorResp(http.StatusInternalServerError, errors.Wrap(err, "error while marshaling remote fs"))
	}
	return &http.Response{
		StatusCode: http.StatusPreconditionFailed,
		Header:     hdr,
		Body:       ioutil.NopCloser(bytes.NewReader(b.Bytes())),
	}
}
