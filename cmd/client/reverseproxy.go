package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

type syncingRoundTripper struct {
	sync       *syncer
	next       http.RoundTripper
	maxRetries int
}

func withSyncingRoundTripper(next http.RoundTripper, sync *syncer) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &syncingRoundTripper{
		next:       next,
		sync:       sync,
		maxRetries: 10}
}

func (s *syncingRoundTripper) RoundTrip(origReq *http.Request) (*http.Response, error) {
	// TODO buffer the request
	// TODO attempt round tripping request
	// TODO compute local checksum, add as header
	localChecksum, err := s.sync.checksum()
	if err != nil {
		return nil, err
	}
	origReq.Header.Set(HdrRundevChecksum, fmt.Sprintf("%d", localChecksum))

	// save request for repeating
	var b bytes.Buffer
	if err := origReq.Write(&b); err != nil {
		return nil, errors.Wrap(err, "failed to buffer request")
	}

	for retry := 0; retry < s.maxRetries; retry++ {
		req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(b.Bytes()))) // probably can be simplified
		if err != nil {
			return nil, errors.Wrap(err, "failed to un-buffer request")
		}
		req.URL = origReq.URL

		// round-trip the request
		resp, err := s.next.RoundTrip(req)
		if err != nil {
			return resp, err
		}
		ct := resp.Header.Get("content-type")
		switch ct {
		case MimeChecksumMismatch:
			log.Printf("[reverse proxy] remote responded with checksum mismatch")
		case MimeDumbRepeat:
			log.Printf("[reverse proxy] remote responded with dumb-repeat")
		default:
			log.Printf("[reverse proxy] request completed on retry=%d", retry)
			return resp, nil
		}
	}
	// TODO check response (checksum mismatch?, build error?, run error?)
	// TODO handle checksum mismatch
	// TODO compute diff, create patch payload
	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       ioutil.NopCloser(strings.NewReader(fmt.Sprintf("max retries exceeded (%d) syncing code", s.maxRetries))),
	}, nil
}
