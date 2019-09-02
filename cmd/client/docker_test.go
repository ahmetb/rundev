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
	"reflect"
	"testing"
)

func Test_parseDockerfileEntrypoint(t *testing.T) {
	tests := []struct {
		name    string
		df      string
		want    cmd
		wantErr bool
	}{
		{
			name:    "nothing",
			df:      `MAINTAINER David Bowie`,
			wantErr: true,
		},
		{
			name: "just cmd (non-json)",
			df:   `CMD /bin/server`,
			want: cmd{"/bin/sh", []string{"-c", "/bin/server"}},
		},
		{
			name: "just cmd (json)",
			df:   `CMD ["/bin/server"]`,
			want: cmd{"/bin/server", []string{}},
		},
		{
			name: "just entrypoint (non-json)",
			df:   `ENTRYPOINT /bin/server`,
			want: cmd{"/bin/sh", []string{"-c", "/bin/server"}},
		},
		{
			name: "repetitive entrypoint",
			df: `ENTRYPOINT ["/bin/date"]
					ENTRYPOINT ["/bin/server"]"`,
			want: cmd{"/bin/server", []string{}},
		},
		{
			name: "repetitive cmd",
			df: `CMD ["/bin/date"]
					CMD ["/bin/server"]"`,
			want: cmd{"/bin/server", []string{}},
		},
		{
			name: "just entrypoint (json)",
			df:   `ENTRYPOINT ["/bin/server"]`,
			want: cmd{"/bin/server", []string{}},
		},
		{
			name: "mix of entrypoint and cmd (both json)",
			df: `ENTRYPOINT ["/bin/server"]
					CMD ["arg1", "arg2"]`,
			want: cmd{"/bin/server", []string{"arg1", "arg2"}},
		},
		{
			name: "mix of entrypoint and cmd (both json), ordering should not matter",
			df: `CMD ["arg1", "arg2"]
					ENTRYPOINT ["/bin/server"]`,
			want: cmd{"/bin/server", []string{"arg1", "arg2"}},
		},
		{
			name: "mix of entrypoint and cmd (both json), earlier CMDs are ignored",
			df: `CMD ["arg0"]
					CMD ["arg1", "arg2"]
					ENTRYPOINT ["/bin/server"]`,
			want: cmd{"/bin/server", []string{"arg1", "arg2"}},
		},
		{
			name: "mix of entrypoint (json) and cmd (non-json)",
			df: `ENTRYPOINT ["/bin/server"]
					CMD arg1 arg2`,
			want: cmd{"/bin/server", []string{"/bin/sh", "-c", "arg1 arg2"}},
		},
		{
			name: "mix of entrypoint (non-json) and cmd (json)",
			df: `ENTRYPOINT /bin/server foo bar
					CMD ["arg1", "arg2"]`, // bad idea, as /bin/sh won't do anything with args after the main arg (that comes after -c)
			want: cmd{"/bin/sh", []string{"-c", "/bin/server foo bar", "arg1", "arg2"}},
		},
		{
			name: "mix of entrypoint and cmd (both non-json)",
			df: `ENTRYPOINT /bin/server foo bar
					CMD arg1 arg2`, // bad idea, user probably already gets an error from their original dockerfile
			want: cmd{"/bin/sh", []string{"-c", "/bin/server foo bar", "/bin/sh", "-c", "arg1 arg2"}},
		},
		{
			name: "multi stage (reset)",
			df: `FROM a1 AS c1
ENTRYPOINT b
CMD c d
FROM a2 as c2
ENTRYPOINT /bin/server`,
			want: cmd{"/bin/sh", []string{"-c", "/bin/server"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df, err := parseDockerfile([]byte(tt.df))
			if err != nil {
				t.Fatalf("parsing dockerfile failed: %v", err)
			}
			got, err := parseEntrypoint(df)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseEntrypoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseEntrypoint() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseBuildCmds(t *testing.T) {
	tests := []struct {
		name string
		df   string
		want []cmd
	}{
		{
			name: "no run cmds",
			df:   "FROM scratch",
			want: nil,
		},
		{
			name: "not annotated run cms",
			df: `FROM scratch
RUN apt-get install \
		-qqy \
		a b c && rm -rf /tmp/foo`,
			want: nil,
		},
		{
			name: "some annotated run cmds",
			df: `
FROM scratch
#rundev
RUN date # xrundev
RUN date # rundevx
RUN date # rundev x
RUN pip install -r requirements.txt        # rundev
RUN ["/src/hack/build.sh"] #rundev
RUN date
`,
			want: []cmd{
				{"/bin/sh", []string{"-c", "pip install -r requirements.txt"}},
				{"/src/hack/build.sh", []string{}},
			},
		},
		{
			name: "multi-stage",
			df: `
FROM foo
RUN date #rundev
FROM bar
RUN xyz 		# rundev
RUN ["uname","-a"] #rundev`,
			want: []cmd{
				{"/bin/sh", []string{"-c", "xyz"}},
				{"uname", []string{"-a"}}},
		},
		{
			name: "multi-line run cmd annotated",
			df: `FROM scratch
RUN apt-get -qqy install \
	git \
	libuv \
	psmisc #rundev`,
			want: []cmd{
				{cmd: "/bin/sh", args: []string{"-c", "apt-get -qqy install \tgit \tlibuv " +
					"\tpsmisc"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df, err := parseDockerfile([]byte(tt.df))
			if err != nil {
				t.Fatalf("parsing dockerfile failed: %v", err)
			}
			if got := parseBuildCmds(df); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseBuildCmds() returned unexpected results:\ngot= %v\nwant=%v", got, tt.want)
			}
		})
	}
}
