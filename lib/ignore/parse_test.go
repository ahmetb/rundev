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
