package virter

import (
	"testing"
)

func TestSplitGlobPrefix(t *testing.T) {
	tests := []struct {
		pattern    string
		wantPrefix string
		wantSuffix string
	}{
		{"/tmp/*.txt", "/tmp", "*.txt"},
		{"foo/bar/*.txt", "foo/bar", "*.txt"},
		{"foo/*/bar.txt", "foo", "*/bar.txt"},
		{"*.txt", ".", "*.txt"},
		{"foo/bar/baz", "foo/bar/baz", ""},
		{"/home/user/project/[abc].txt", "/home/user/project", "[abc].txt"},
		{"foo/?/bar", "foo", "?/bar"},
	}

	for _, tc := range tests {
		t.Run(tc.pattern, func(t *testing.T) {
			prefix, suffix := splitGlobPrefix(tc.pattern)
			if prefix != tc.wantPrefix {
				t.Errorf("splitGlobPrefix(%q) prefix = %q, want %q", tc.pattern, prefix, tc.wantPrefix)
			}
			if suffix != tc.wantSuffix {
				t.Errorf("splitGlobPrefix(%q) suffix = %q, want %q", tc.pattern, suffix, tc.wantSuffix)
			}
		})
	}
}

func TestCheckPathInWorkDir(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		workDir string
		wantErr bool
	}{
		{"subdir", "/home/user/project/subdir/file", "/home/user/project", false},
		{"same dir", "/home/user/project/file", "/home/user/project", false},
		{"workdir itself", "/home/user/project", "/home/user/project", false},
		{"parent escape", "/home/user/other/file", "/home/user/project", true},
		{"root escape", "/etc/shadow", "/home/user/project", true},
		{"prefix trick", "/home/user/projectevil/file", "/home/user/project", true},
		{"parent dots", "/home/user/project/../other/file", "/home/user/project", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkPathInWorkDir(tc.path, tc.workDir)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for path %q in workdir %q", tc.path, tc.workDir)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for path %q in workdir %q: %v", tc.path, tc.workDir, err)
			}
		})
	}
}
