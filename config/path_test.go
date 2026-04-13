package config

import "testing"

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		remote       string
		wantHost     string
		wantPath     string
		wantLocalDir string
		wantErr      bool
	}{
		{
			remote:       "git@gitlab.example.com:org/services/myrepo.git",
			wantHost:     "gitlab.example.com",
			wantPath:     "org/services/myrepo",
			wantLocalDir: "gitlab.example.com/org/services/myrepo",
		},
		{
			remote:       "git@github.com:example/myrepo.git",
			wantHost:     "github.com",
			wantPath:     "example/myrepo",
			wantLocalDir: "github.com/example/myrepo",
		},
		{
			remote:       "https://github.com/example/myrepo.git",
			wantHost:     "github.com",
			wantPath:     "example/myrepo",
			wantLocalDir: "github.com/example/myrepo",
		},
		{
			remote:       "https://gitlab.example.com/org/services/myrepo.git",
			wantHost:     "gitlab.example.com",
			wantPath:     "org/services/myrepo",
			wantLocalDir: "gitlab.example.com/org/services/myrepo",
		},
		{
			remote:  "not-a-valid-remote",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.remote, func(t *testing.T) {
			host, path, localDir, err := ParseRemoteURL(tt.remote)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRemoteURL(%q) error = %v, wantErr %v", tt.remote, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if host != tt.wantHost {
				t.Errorf("host = %q, want %q", host, tt.wantHost)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
			if localDir != tt.wantLocalDir {
				t.Errorf("localDir = %q, want %q", localDir, tt.wantLocalDir)
			}
		})
	}
}
