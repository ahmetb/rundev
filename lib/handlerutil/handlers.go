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

// Package handlerutil provides common handlers between
// rundev client and daemon.
package handlerutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ahmetb/rundev/lib/constants"
	"github.com/ahmetb/rundev/lib/fsutil"
	"github.com/ahmetb/rundev/lib/ignore"
	"io"
	"net/http"
)

func NewFSDebugHandler(dir string, ignores *ignore.FileIgnores) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		i := ignores
		if _, ok := req.URL.Query()["full"] ; ok{
			i = nil // ?full disables the file exclusion rules
		}

		fs, err := fsutil.Walk(dir, i)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w,"failed to fetch local filesystem: %+v", err)
			return
		}
		w.Header().Set(constants.HdrRundevChecksum, fmt.Sprintf("%v", fs.RootChecksum()))
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetIndent("", "  ")
		if err := enc.Encode(fs); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w,"failed to encode json: %+v", err)
			return
		}
		io.Copy(w,&b)
	}
}


// NewUnsupportedDebugEndpointHandler returns a 404 handler for debug
// paths to prevent falling back to reverse proxy.
func NewUnsupportedDebugEndpointHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "not found: debug endpoint %s does not exist.", req.URL.Path)
	}
}
