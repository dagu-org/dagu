package logger

import (
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/utils"
)

func TestSimpleLogger(t *testing.T) {
	tmpDir := utils.MustTempDir("test-simple-logger")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	rl := NewSimpleLogger(tmpDir, "test", time.Millisecond*200)
	rl.Open()

	_, err := rl.Write([]byte("test log\n"))
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 250)

	_, err = rl.Write([]byte("test log2\n"))
	require.NoError(t, err)

	_ = rl.Close()
	time.Sleep(time.Millisecond * 100)

	f, err := os.Open(tmpDir)
	require.NoError(t, err)
	defer func() {
		_ = f.Close()
	}()

	fis, _ := f.Readdir(0)
	require.Equal(t, 2, len(fis))
	for _, fi := range fis {
		require.Regexp(t, "test\\d{8}.\\d{2}:\\d{2}:\\d{2}.\\d{3}.log", fi.Name())
	}

	b, _ := os.ReadFile(path.Join(tmpDir, fis[0].Name()))
	require.Equal(t, "test log\n", string(b))

	b, _ = os.ReadFile(path.Join(tmpDir, fis[1].Name()))
	require.Equal(t, "test log2\n", string(b))
}

func TestTimeToSwitchLog(t *testing.T) {
	rl := NewSimpleLogger("", "test", time.Hour*24)
	tm := rl.timeToSwitchLog()
	d := time.Hour*24 - time.Until(tm)
	require.GreaterOrEqual(t, d, time.Duration(0))
	require.LessOrEqual(t, time.Second, d)
}
