package main

import (
	"bytes"
	"fmt"
	"github.com/ahmetb/rundev/lib/constants"
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

func (s *syncingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// TODO buffer the request
	// TODO attempt round tripping request
	// TODO compute local checksum, add as header
	localChecksum, err := s.sync.checksum()
	if err != nil {
		return nil, err
	}
	req.Header.Set(constants.HdrRundevChecksum, fmt.Sprintf("%d", localChecksum))

	// save request for repeating
	var body []byte
	if req.Body != nil {
		body, err = ioutil.ReadAll(req.Body)
		defer req.Body.Close()
		if err != nil {
			return nil, errors.Wrap(err, "failed to buffer request body")
		}
	}
	for retry := 0; retry < s.maxRetries; retry++ {
		if body != nil {
			req.Body = ioutil.NopCloser(bytes.NewReader(body))
		}

		// round-trip the request
		resp, err := s.next.RoundTrip(req)
		if err != nil {
			return resp, err
		}
		ct := resp.Header.Get("content-type")
		switch ct {
		case constants.MimeDumbRepeat:
			log.Printf("[reverse proxy] remote responded with dumb-repeat")
		case constants.MimeChecksumMismatch:
			log.Printf("[reverse proxy] remote responded with checksum mismatch")
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
