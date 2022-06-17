package views

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

func TestMain(m *testing.M) {
	tmpDir := utils.MustTempDir("test-views")
	os.Setenv("HOST", "localhost")
	settings.InitTest(tmpDir)
	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

func TestView(t *testing.T) {
	viewsDir := settings.MustGet(settings.CONFIG__VIEWS_DIR)
	defer func() {
		os.RemoveAll(viewsDir)
	}()

	view := &View{
		Name:        "",
		ContainTags: []string{"a", "b"},
	}

	err := SaveView(view)
	require.EqualError(t, err, ErrInvalidName.Error())

	view = &View{
		Name:        "test",
		ContainTags: []string{"a", "b"},
	}

	err = SaveView(view)
	require.NoError(t, err)

	require.True(t, utils.FileExists(path.Join(viewsDir, "test.json")))

	views := GetViews()
	require.Equal(t, 1, len(views))
	require.Equal(t, view, views[0])

	v2, err := GetView("test")
	require.NoError(t, err)
	require.Equal(t, view, v2)

	err = DeleteView(v2)
	require.NoError(t, err)

	_, err = GetView("test")
	require.Error(t, err)
}

func TestViewMarshaling(t *testing.T) {
	v := &View{
		Name:        "test",
		ContainTags: []string{"a", "b"},
	}
	js, err := v.ToJson()
	require.NoError(t, err)

	v2, err := ViewFromJson(js)
	require.NoError(t, err)
	require.Equal(t, v, v2)
}
