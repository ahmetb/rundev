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

package types

type ProcError struct {
	Message string `json:"message"`
	Output  string `json:"output"`
}

type Cmd []string

func (c Cmd) Command() string {
	if len(c) == 0 {
		return ""
	}
	return c[0]
}

func (c Cmd) Args() []string {
	if len(c) <= 1 {
		return nil
	}
	return c[1:]
}

type BuildCmd struct {
	C  Cmd      `json:"c"`
	On []string `json:"on,omitempty"` // file patterns
}

type BuildCmds []BuildCmd
