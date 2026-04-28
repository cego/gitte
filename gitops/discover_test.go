package gitops

import "testing"

func TestDedupRepos(t *testing.T) {
	tests := []struct {
		name string
		in   []DiscoveredRepo
		want []DiscoveredRepo
	}{
		{
			name: "no duplicates",
			in: []DiscoveredRepo{
				{Remote: "git@a:x/y.git", Host: "a", Path: "x/y"},
				{Remote: "git@a:x/z.git", Host: "a", Path: "x/z"},
			},
			want: []DiscoveredRepo{
				{Remote: "git@a:x/y.git", Host: "a", Path: "x/y"},
				{Remote: "git@a:x/z.git", Host: "a", Path: "x/z"},
			},
		},
		{
			name: "duplicate host+path keeps first",
			in: []DiscoveredRepo{
				{Remote: "git@a:x/y.git", Host: "a", Path: "x/y"},
				{Remote: "https://a/x/y.git", Host: "a", Path: "x/y"},
			},
			want: []DiscoveredRepo{
				{Remote: "git@a:x/y.git", Host: "a", Path: "x/y"},
			},
		},
		{
			name: "same path different host kept",
			in: []DiscoveredRepo{
				{Remote: "git@a:x/y.git", Host: "a", Path: "x/y"},
				{Remote: "git@b:x/y.git", Host: "b", Path: "x/y"},
			},
			want: []DiscoveredRepo{
				{Remote: "git@a:x/y.git", Host: "a", Path: "x/y"},
				{Remote: "git@b:x/y.git", Host: "b", Path: "x/y"},
			},
		},
		{
			name: "empty",
			in:   nil,
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupRepos(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got %d, want %d (%v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("index %d: got %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}
