package dockerfile

import (
	"reflect"
	"testing"
)

func TestParseDockerfileEntrypoint(t *testing.T) {
	tests := []struct {
		name    string
		df      string
		want    Cmd
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
			want: Cmd{"/bin/sh", []string{"-c", "/bin/server"}},
		},
		{
			name: "just cmd (json)",
			df:   `CMD ["/bin/server"]`,
			want: Cmd{"/bin/server", []string{}},
		},
		{
			name: "just entrypoint (non-json)",
			df:   `ENTRYPOINT /bin/server`,
			want: Cmd{"/bin/sh", []string{"-c", "/bin/server"}},
		},
		{
			name: "repetitive entrypoint",
			df: `ENTRYPOINT ["/bin/date"]
					ENTRYPOINT ["/bin/server"]"`,
			want: Cmd{"/bin/server", []string{}},
		},
		{
			name: "repetitive cmd",
			df: `CMD ["/bin/date"]
					CMD ["/bin/server"]"`,
			want: Cmd{"/bin/server", []string{}},
		},
		{
			name: "just entrypoint (json)",
			df:   `ENTRYPOINT ["/bin/server"]`,
			want: Cmd{"/bin/server", []string{}},
		},
		{
			name: "mix of entrypoint and cmd (both json)",
			df: `ENTRYPOINT ["/bin/server"]
					CMD ["arg1", "arg2"]`,
			want: Cmd{"/bin/server", []string{"arg1", "arg2"}},
		},
		{
			name: "mix of entrypoint and cmd (both json), ordering should not matter",
			df: `CMD ["arg1", "arg2"]
					ENTRYPOINT ["/bin/server"]`,
			want: Cmd{"/bin/server", []string{"arg1", "arg2"}},
		},
		{
			name: "mix of entrypoint and cmd (both json), earlier CMDs are ignored",
			df: `CMD ["arg0"]
					CMD ["arg1", "arg2"]
					ENTRYPOINT ["/bin/server"]`,
			want: Cmd{"/bin/server", []string{"arg1", "arg2"}},
		},
		{
			name: "mix of entrypoint (json) and cmd (non-json)",
			df: `ENTRYPOINT ["/bin/server"]
					CMD arg1 arg2`,
			want: Cmd{"/bin/server", []string{"/bin/sh", "-c", "arg1 arg2"}},
		},
		{
			name: "mix of entrypoint (non-json) and cmd (json)",
			df: `ENTRYPOINT /bin/server foo bar
					CMD ["arg1", "arg2"]`, // bad idea, as /bin/sh won't do anything with args after the main arg (that comes after -c)
			want: Cmd{"/bin/sh", []string{"-c", "/bin/server foo bar", "arg1", "arg2"}},
		},
		{
			name: "mix of entrypoint and cmd (both non-json)",
			df: `ENTRYPOINT /bin/server foo bar
					CMD arg1 arg2`, // bad idea, user probably already gets an error from their original dockerfile
			want: Cmd{"/bin/sh", []string{"-c", "/bin/server foo bar", "/bin/sh", "-c", "arg1 arg2"}},
		},
		{
			name: "multi stage (reset)",
			df: `FROM a1 AS c1
ENTRYPOINT b
CMD c d
FROM a2 as c2
ENTRYPOINT /bin/server`,
			want: Cmd{"/bin/sh", []string{"-c", "/bin/server"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df, err := ParseDockerfile([]byte(tt.df))
			if err != nil {
				t.Fatalf("parsing dockerfile failed: %v", err)
			}
			got, err := ParseEntrypoint(df)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEntrypoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseEntrypoint() got = %v, want %v", got, tt.want)
			}
		})
	}
}
