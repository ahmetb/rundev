package dockerfile

import (
	"github.com/ahmetb/rundev/lib/types"
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestParseBuildCmds(t *testing.T) {
	tests := []struct {
		name string
		df   string
		want types.BuildCmds
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
			want: []types.BuildCmd{
				{C: []string{"/bin/sh", "-c", "pip install -r requirements.txt"}},
				{C: []string{"/src/hack/build.sh"}},
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
			want: []types.BuildCmd{
				{C: []string{"/bin/sh", "-c", "xyz"}},
				{C: []string{"uname", "-a"}}},
		},
		{
			name: "multi-line run cmd annotated",
			df: `FROM scratch
RUN apt-get -qqy install \
	git \
	libuv \
	psmisc #rundev`,
			want: []types.BuildCmd{
				{
					C: []string{"/bin/sh", "-c",
						"apt-get -qqy install \tgit \tlibuv " +
							"\tpsmisc"}},
			},
		},
		{
			name: "run cmd with conditions",
			df:   `RUN ["foo"] #rundev[requirements.txt,**.py]`,
			want: []types.BuildCmd{
				{
					C:  []string{"foo"},
					On: []string{"requirements.txt", "**.py"},
				},
			},
		},
		{
			name: "run cmd with empty conditions default to nil",
			df:   `RUN ["date"] #rundev[]`,
			want: []types.BuildCmd{{C: []string{"date"}}},
		},
		{
			name: "space before conditions",
			df:   `RUN ["date"] # rundev [foo]`,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df, err := ParseDockerfile([]byte(tt.df))
			if err != nil {
				t.Fatalf("parsing dockerfile failed: %v", err)
			}
			got := ParseBuildCmds(df)

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ParseBuildCmds() returned unexpected results:\n%s", diff)
			}
		})
	}
}
