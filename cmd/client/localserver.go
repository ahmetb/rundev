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
	"github.com/ahmetb/rundev/lib/handlerutil"
	"github.com/kr/pretty"
	"github.com/pkg/errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

type localServerOpts struct {
	sync        *syncer
	proxyTarget string
}

type localServer struct {
	opts localServerOpts
}

func newLocalServer(opts localServerOpts) (http.Handler, error) {
	ls := &localServer{opts: opts}

	reverseProxy, err := newReverseProxyHandler(opts.proxyTarget, ls.opts.sync)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize reverse proxy")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rundev/fsz", handlerutil.NewFSDebugHandler(ls.opts.sync.opts.localDir, ls.opts.sync.opts.ignores))
	mux.HandleFunc("/rundev/debugz", ls.debugHandler)
	mux.HandleFunc("/rundev/", handlerutil.NewUnsupportedDebugEndpointHandler())
	mux.HandleFunc("/favicon.ico", handlerutil.NewUnsupportedDebugEndpointHandler()) // TODO(ahmetb) annoyance during testing on browser
	// TODO(ahmetb) add /rundev/syncz
	mux.Handle("/", reverseProxy)
	return mux, nil
}

func newReverseProxyHandler(addr string, sync *syncer) (http.Handler, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse remote addr as url %s", addr)
	}
	rp := httputil.NewSingleHostReverseProxy(u)
	rp.Transport = withSyncingRoundTripper(rp.Transport, sync, u.Host)
	return rp, nil
}

func (srv *localServer) debugHandler(w http.ResponseWriter, req *http.Request) {
	checksum, err := srv.opts.sync.checksum()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Errorf("failed to fetch local filesystem: %+v", err)
	}
	fmt.Fprintf(w, "fs checksum: %v\n", checksum)
	fmt.Fprintf(w, "pid: %d\n", os.Getpid())
	wd, _ := os.Getwd()
	fmt.Fprintf(w, "cwd: %s\n", wd)
	fmt.Fprint(w, "sync:\n")
	fmt.Fprintf(w, "  dir: %# v\n", pretty.Formatter(srv.opts.sync.opts.localDir))
	fmt.Fprintf(w, "  target: %# v\n", pretty.Formatter(srv.opts.sync.opts.targetAddr))
	fmt.Fprintf(w, "  ignores: %# v\n", pretty.Formatter(srv.opts.sync.opts.ignores))
}
