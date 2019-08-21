// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"time"
)

type syncingRoundTripper struct {
	sync       *syncer
	next       http.RoundTripper
	maxRetries int
	hostHdr    string
}

func withSyncingRoundTripper(next http.RoundTripper, sync *syncer, host string) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &syncingRoundTripper{
		next:       next,
		sync:       sync,
		maxRetries: 10,
		hostHdr:    host}
}

func (s *syncingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	log.Printf("[reverse proxy] request received path=%s method=%s", req.URL.Path, req.Method)
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
		req.Host = s.hostHdr
		req.Header.Set("Host", s.hostHdr)

		// round-trip the request
		if retry != 0 {
			log.Printf("[reverse proxy] repeating request n=%d path=%s method=%s", retry, req.URL.Path, req.Method)
		}
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
			// only for testing purposes
			log.Printf("[reverse proxy] remote responded with dumb-repeat")
		case constants.MimeChecksumMismatch:
			remoteSum := resp.Header.Get(constants.HdrRundevChecksum)
			log.Printf("[reverse proxy] remote responded with checksum mismatch (%s)", remoteSum)
			remoteFS, err := parseMismatchResponse(resp.Body)
			if err != nil {
				return nil, errors.Wrap(err, "failed to read remote fs in the response") // TODO mkErrorResp here
			}
			if err := s.sync.uploadPatch(remoteFS, remoteSum); err != nil {
				log.Printf("[retry %d] sync was failed: %v", retry, err)
				continue
			}
		default:
			log.Printf("[reverse proxy] request completed on retry=%d path=%s status=%d took=%v (%s)", retry, req.URL.Path, resp.StatusCode, time.Since(start), resp.Header.Get("content-type"))
			return resp, nil
		}
	}

	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body: ioutil.NopCloser(strings.NewReader(fmt.Sprintf("rundev tried %d times syncing code, but it was still getting a checksum mismatch.\n"+
			"please report an issue with console logs, /rundev/fsz and /rundevd/fsz responses.", s.maxRetries))),
	}, nil
}
