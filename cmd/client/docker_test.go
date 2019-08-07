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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDockerfileEntrypoint([]byte(tt.df))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDockerfileEntrypoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseDockerfileEntrypoint() got = %v, want %v", got, tt.want)
			}
		})
	}
}
