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
	"fmt"
	"github.com/ahmetb/rundev/lib/constants"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReverseProxy_transmitsChecksum(t *testing.T) {
	visits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		v := req.Header.Get(constants.HdrRundevChecksum)
		if v == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "empty checksum header")
			return
		}
		visits++
		fmt.Fprintf(w, "checksum header: %s", v)
	}))
	defer srv.Close()

	syncer := newSyncer(syncOpts{
		localDir: "/Users/ahmetb/workspace/junk/py-hello", // TODO(ahmetb) create tempdir for test
	})

	rp, err := newReverseProxyHandler(srv.URL, syncer)
	if err != nil {
		t.Fatal(err)
	}
	rs := httptest.NewServer(rp)
	defer rs.Close()

	resp, err := http.Get(rs.URL + "/foo")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status code: %d", resp.StatusCode)
	}
	if visits != 1 {
		t.Fatalf("%d visits recorded", visits)
	}
}

func TestReverseProxy_repeatsRequest(t *testing.T) {
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		i++
		if i <= 2 {
			w.Header().Set("content-type", constants.MimeDumbRepeat)
			return
		}
		fmt.Fprintf(w, "done")
	}))
	defer srv.Close()
	syncer := newSyncer(syncOpts{
		localDir: "/Users/ahmetb/workspace/junk/py-hello", // TODO(ahmetb) create tempdir for test
	})
	rp, err := newReverseProxyHandler(srv.URL, syncer)
	if err != nil {
		t.Fatal(err)
	}
	rs := httptest.NewServer(rp)
	defer rs.Close()
	resp, err := http.Get(rs.URL + "/foo")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if i != 3 {
		t.Fatalf("unexpected amount of requests: %d", i)
	}

}
