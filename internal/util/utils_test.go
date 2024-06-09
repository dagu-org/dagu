package util_test

import (
	"bytes"
	"errors"
	"io"
	"log"
	"os"
	"path"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/constants"
	"github.com/dagu-dev/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

func TestMustGetUserHomeDir(t *testing.T) {
	err := os.Setenv("HOME", "/test")
	if err != nil {
		t.Fatal(err)
	}
	hd := util.MustGetUserHomeDir()
	require.Equal(t, "/test", hd)
}

func TestMustGetwd(t *testing.T) {
	wd, _ := os.Getwd()
	require.Equal(t, util.MustGetwd(), wd)
}

func TestFormatTime(t *testing.T) {
	tm := time.Date(2022, 2, 1, 2, 2, 2, 0, time.Now().Location())
	formatted := util.FormatTime(tm)
	require.Equal(t, "2022-02-01 02:02:02", formatted)

	parsed, err := util.ParseTime(formatted)
	require.NoError(t, err)
	require.Equal(t, tm, parsed)

	require.Equal(t, constants.TimeEmpty, util.FormatTime(time.Time{}))
	parsed, err = util.ParseTime(constants.TimeEmpty)
	require.NoError(t, err)
	require.Equal(t, time.Time{}, parsed)
}

func TestFormatDuration(t *testing.T) {
	dr := time.Second*5 + time.Millisecond*100
	require.Equal(t, "5.1s", util.FormatDuration(dr, ""))
}

func TestSplitCommand(t *testing.T) {
	command := "ls -al test/"
	program, args := util.SplitCommand(command, false)
	require.Equal(t, "ls", program)
	require.Equal(t, "-al", args[0])
	require.Equal(t, "test/", args[1])
}

func TestSplitCommandJSON(t *testing.T) {
	command := `echo {\"key\":\"value\"}`
	program, args := util.SplitCommand(command, true)
	require.Equal(t, "echo", program)
	require.Equal(t, "{\"key\":\"value\"}", args[0])
}

func TestFileExits(t *testing.T) {
	require.True(t, util.FileExists("/"))
}

func TestValidFilename(t *testing.T) {
	f := util.ValidFilename("file\\name", "_")
	require.Equal(t, f, "file_name")
}

func TestOpenFile(t *testing.T) {
	tmp, err := os.MkdirTemp("", "open")
	require.NoError(t, err)

	name := path.Join(tmp, "/file.txt")
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)

	defer func() {
		_ = f.Close()
		_ = os.Remove(name)
	}()

	_, _ = f.WriteString("test")
	_ = f.Sync()
	_ = f.Close()

	_, err = util.OpenFile(name)
	require.NoError(t, err)
}

func TestOpenOrCreateFile(t *testing.T) {
	tmp, err := os.MkdirTemp("", "open_or_create")
	require.NoError(t, err)

	name := path.Join(tmp, "/file.txt")
	f, err := util.OpenOrCreateFile(name)
	require.NoError(t, err)

	defer func() {
		_ = f.Close()
		_ = os.Remove(name)
	}()

	require.True(t, util.FileExists(name))

	_ = f.Close()
	_ = os.Remove(name)

	_, err = util.OpenFile(name)
	require.Error(t, err)
}

func TestParseVariable(t *testing.T) {
	_ = os.Setenv("TEST_VAR", "test")
	r, err := util.ParseVariable("${TEST_VAR}")
	require.NoError(t, err)
	require.Equal(t, r, "test")

	_, err = util.ParseVariable("`ech test`")
	require.Error(t, err)

	r, err = util.ParseVariable("`echo test`")
	require.NoError(t, err)
	require.Equal(t, r, "test")
}

func TestMustTempDir(t *testing.T) {
	dir := util.MustTempDir("tempdir")
	defer os.RemoveAll(dir)
	require.Contains(t, dir, "tempdir")
}

func TestOpenfile(t *testing.T) {
	dir := util.MustTempDir("tempdir")
	defer os.RemoveAll(dir)

	fn := path.Join(dir, "test.txt")
	f, err := util.OpenOrCreateFile(fn)
	require.NoError(t, err)
	defer f.Close()

	_, err = f.WriteString("test")
	require.NoError(t, err)
	require.NoError(t, f.Sync(), err)
	require.NoError(t, f.Close(), err)
	require.True(t, util.FileExists(fn))

	f2, err := os.Open(fn)
	require.NoError(t, err)
	defer f2.Close()
	b, err := io.ReadAll(f2)
	require.NoError(t, err)
	require.Equal(t, "test", string(b))
}

func TestIgnoreErr(t *testing.T) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	log.SetOutput(w)

	defer func() {
		os.Stdout = origStdout
		log.SetOutput(origStdout)
	}()

	util.LogErr("test action", errors.New("test error"))
	os.Stdout = origStdout
	_ = w.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	s := buf.String()
	require.Contains(t, s, "test action failed")
	require.Contains(t, s, "test error")
}

func TestTruncString(t *testing.T) {
	require.Equal(t, "12345678", util.TruncString("123456789", 8))
	require.Equal(t, "1234567", util.TruncString("1234567", 8))
}

func TestStringWithFallback(t *testing.T) {
	require.Equal(t, "test", util.StringWithFallback("", "test"))
	require.Equal(t, "test", util.StringWithFallback("test", "fallback"))
}

func TestMatchExtension(t *testing.T) {
	require.True(t, util.MatchExtension("test.txt", []string{".csv", ".txt"}))
	require.False(t, util.MatchExtension("test.txt", []string{".csv"}))
}

func TestFixedTIme(t *testing.T) {
	tm := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	util.SetFixedTime(tm)
	require.Equal(t, tm, util.Now())
	util.SetFixedTime(time.Time{})
	require.NotEqual(t, tm, util.Now())
}

func TestParseParams(t *testing.T) {
	val := "QUESTION=\"what is your favorite activity?\""
	ret, err := util.ParseParams(val, true)
	require.NoError(t, err)
	require.Equal(t, 1, len(ret))
	require.Equal(t, ret[0].Name, "QUESTION")
	require.Equal(t, ret[0].Value, "what is your favorite activity?")
}
