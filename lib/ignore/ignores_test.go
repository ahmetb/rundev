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

import "testing"

func TestIgnored(t *testing.T) {
	type args struct {
		path       string
		exclusions []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "direct match",
			args: args{
				path:       "a/b",
				exclusions: []string{"a/b"},
			},
			want: true,
		},
		{
			name: "direct match, single glob",
			args: args{
				path:       "a/b",
				exclusions: []string{"a/*"},
			},
			want: true,
		},
		{
			name: "direct match, single glob in multi-nest",
			args: args{
				path:       "a/b/c",
				exclusions: []string{"a/*"},
			},
			want: false,
		},
		{
			name: "direct match, nested glob",
			args: args{
				path:       "a/b/c",
				exclusions: []string{"a/*/*"},
			},
			want: true,
		},
		{
			name: "direct match, double-star",
			args: args{
				path:       "a/b/c",
				exclusions: []string{"a/**"},
			},
			want: true,
		},
		{
			name: "leading slash in pattern",
			args: args{
				path:       "a/b",
				exclusions: []string{"/a/*"},
			},
			want: true,
		},
		{
			name: "sub-path is excluded if dir is excluded without star",
			args: args{
				path:       "__pycache__/foo",
				exclusions: []string{"__pycache__"},
			},
			want: false,
		},
		{
			name: "extension match with double-star",
			args: args{
				path:       "a/b/c/foo.py",
				exclusions: []string{"**/*.py"},
			},
			want: true,
		},
		{
			name: "no match",
			args: args{
				path:       "code.go",
				exclusions: []string{"**.py"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ignored(tt.args.path, tt.args.exclusions); got != tt.want {
				t.Errorf("Ignored() = %v, want %v", got, tt.want)
			}
		})
	}
}
