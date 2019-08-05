package main

import (
	"bytes"
	"encoding/json"
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
			return nil, err // TODO(ahmetb) returning err from roundtrip method is not surfacing the error message in the response body, and prints a log to stderr by net/http's internal logger
		}
		ct := resp.Header.Get("content-type")
		switch ct {
		case constants.MimeProcessError:
			log.Printf("[reverse proxy] remote responded with process error")
			var pe constants.ProcError
			if err := json.NewDecoder(resp.Body).Decode(&pe); err != nil {
				if resp.Body != nil {
					resp.Body.Close()
				}
				return nil, errors.Wrap(err, "failed to parse proc error response body") // TODO ahmetb mkErrorResp here
			}
			resp.Body.Close()
			return &http.Response{
				StatusCode: resp.StatusCode,
				Body:       ioutil.NopCloser(strings.NewReader(fmt.Sprintf("process error: %s\n\noutput:\n%s", pe.Message, pe.Output))),
			}, nil
		case constants.MimeDumbRepeat:
			log.Printf("[reverse proxy] remote responded with dumb-repeat")
		case constants.MimeChecksumMismatch:
			remoteSum := resp.Header.Get(constants.HdrRundevChecksum)
			log.Printf("[reverse proxy] remote responded with checksum mismatch (%s)", remoteSum)
			remoteFS, err := parseMismatchResponse(resp.Body)
			if err != nil {
				return nil, errors.Wrap(err, "failed to read remote fs in the response") // TODO mkErrorResp here
			}
			if err := s.sync.uploadPatch(remoteFS, remoteSum); err != nil {
				log.Printf("[retry %d] sync was failed: %+v", retry, err)
				continue
			}
		default:
			log.Printf("[reverse proxy] request completed on retry=%d path=%s (%s)", retry, req.URL.Path, resp.Header.Get("content-type"))
			return resp, nil
		}
	}

	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       ioutil.NopCloser(strings.NewReader(fmt.Sprintf("max retries exceeded (%d) syncing code", s.maxRetries))),
	}, nil
}
