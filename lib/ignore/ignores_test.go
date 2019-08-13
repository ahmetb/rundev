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
