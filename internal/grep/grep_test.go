package grep

import (
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/utils"
)

func TestGrep(t *testing.T) {
	dir := path.Join(utils.MustGetwd(), "/testdata")
	for _, tc := range []struct {
		Name    string
		File    string
		Pattern string
		Opts    *Options
		Want    map[int]string
		IsErr   bool
	}{
		{
			Name:    "simple",
			File:    path.Join(dir, "test.txt"),
			Pattern: "b",
			Want:    map[int]string{2: "bb"},
		},
		{
			Name:    "regexp",
			File:    path.Join(dir, "test.txt"),
			Pattern: "^b.",
			Opts: &Options{
				Regexp: true,
			},
			Want: map[int]string{2: "bb"},
		},
		{
			Name:    "before",
			File:    path.Join(dir, "test.txt"),
			Pattern: "b",
			Opts: &Options{
				Before: 1,
			},
			Want: map[int]string{2: "aa\nbb"},
		},
		{
			Name:    "before+after",
			File:    path.Join(dir, "test.txt"),
			Pattern: "cc",
			Opts: &Options{
				Before: 2,
				After:  2,
			},
			Want: map[int]string{3: "aa\nbb\ncc\ndd\nee"},
		},
		{
			Name:    "before+after,firstline",
			File:    path.Join(dir, "test.txt"),
			Pattern: "aa",
			Opts: &Options{
				Before: 1,
				After:  1,
			},
			Want: map[int]string{1: "aa\nbb"},
		},
		{
			Name:    "before+after,lastline",
			File:    path.Join(dir, "test.txt"),
			Pattern: "ee",
			Opts: &Options{
				Before: 1,
				After:  1,
			},
			Want: map[int]string{5: "dd\nee"},
		},
		{
			Name:    "no file",
			File:    path.Join(dir, "dummy.txt"),
			Pattern: "aa",
			IsErr:   true,
		},
		{
			Name:    "no pattern",
			File:    path.Join(dir, "test.txt"),
			Pattern: "",
			IsErr:   true,
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			ret, err := Grep(tc.File, tc.Pattern, tc.Opts)
			if tc.IsErr {
				require.Empty(t, ret)
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.Want, ret)
		})
	}
}
