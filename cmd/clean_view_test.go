package cmd

import "testing"

func TestColsForWidth(t *testing.T) {
	cases := []struct {
		width int
		want  int
	}{
		{79, 1},
		{80, 1},
		{119, 1},
		{120, 2},
		{179, 2},
		{180, 3},
		{300, 3},
	}
	for _, c := range cases {
		got := colsForWidth(c.width)
		if got != c.want {
			t.Errorf("colsForWidth(%d) = %d, want %d", c.width, got, c.want)
		}
	}
}
