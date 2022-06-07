package storage

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/utils"
)

func TestStorage(t *testing.T) {
	tmpDir := utils.MustTempDir("test-storage")
	defer os.RemoveAll(tmpDir)

	f, _ := utils.CreateFile(path.Join(tmpDir, "test.json"))
	_, _ = f.WriteString("{ \"Name\": \"test\" }")
	f.Sync()
	f.Close()

	s := &Storage{
		tmpDir,
	}
	fis, err := s.List()
	require.NoError(t, err)

	require.Equal(t, 1, len(fis))
	require.Equal(t, "test.json", fis[0].Name())

	_, err = s.Read(fis[0].Name())
	require.NoError(t, err)

	b, _ := (&models.View{Name: "test2"}).ToJson()
	err = s.Save(fis[0].Name(), b)
	require.NoError(t, err)

	v, err := models.ViewFromJson(string(b))
	require.NoError(t, err)
	require.Equal(t, v.Name, "test2")

	err = s.Delete(fis[0].Name())
	require.NoError(t, err)
	require.False(t, utils.FileExists(path.Join(tmpDir, "test.json")))
}
