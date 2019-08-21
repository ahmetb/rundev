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

package ignore

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestParseDockerignore(t *testing.T) {
	type args struct {
		r io.Reader
	}
	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{
			name:    "empty file",
			in:      "",
			want:    nil,
			wantErr: false,
		},
		{
			name: "good parse",
			in: `# comment
a
b/c
d/**
e/*f/*
g?`,
			want:    []string{"a", "b/c", "d/**", "e/*f/*", "g?"},
			wantErr: false,
		},
		{
			name: "exception rules not supported",
			in: `node_modules/**
!node_modules/package.json`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "bad pattern",
			in:      "[-]",
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDockerignore(strings.NewReader(tt.in))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDockerignore() error = %v, wantErr %v, got=%v", err, tt.wantErr, got)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseDockerignore() got = %v, want %v", got, tt.want)
			}
		})
	}
}
