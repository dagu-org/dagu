package storage

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/utils"
)

func TestStorage(t *testing.T) {
	tmpDir := utils.MustTempDir("test-storage")
	defer os.RemoveAll(tmpDir)

	// create data for test
	data := "{ \"Name\": \"test\" }"
	f, _ := utils.CreateFile(path.Join(tmpDir, "test.json"))
	_, _ = f.WriteString(data)
	f.Sync()
	f.Close()

	// confirm data is saved
	s := &Storage{tmpDir}
	fis, err := s.List()
	require.NoError(t, err)

	require.Equal(t, 1, len(fis))
	require.Equal(t, "test.json", fis[0].Name())

	// save data with same name
	data2 := "{ \"Name\": \"test\" }"
	err = s.Save(fis[0].Name(), []byte(data2))
	require.NoError(t, err)

	// confirm data is overwritten
	b2 := s.MustRead(fis[0].Name())
	require.Equal(t, data2, string(b2))

	// test delete
	err = s.Delete(fis[0].Name())
	require.NoError(t, err)
	require.False(t, utils.FileExists(path.Join(tmpDir, "test.json")))
}
