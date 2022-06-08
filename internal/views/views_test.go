package views

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

func TestView(t *testing.T) {
	viewsDir := settings.MustGet(settings.CONFIG__VIEWS_DIR)
	defer func() {
		os.RemoveAll(viewsDir)
	}()

	view := &models.View{
		Name:        "test",
		ContainTags: []string{"a", "b"},
	}

	err := SaveView(view)
	require.NoError(t, err)

	require.True(t, utils.FileExists(path.Join(viewsDir, "test.json")))

	views := GetViews()
	require.Equal(t, 1, len(views))
	require.Equal(t, view, views[0])
}
