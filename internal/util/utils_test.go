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

func Test_MustGetUserHomeDir(t *testing.T) {
	t.Run("get home dir", func(t *testing.T) {
		err := os.Setenv("HOME", "/test")
		if err != nil {
			t.Fatal(err)
		}
		hd := util.MustGetUserHomeDir()
		require.Equal(t, "/test", hd)
	})
}

func Test_MustGetwd(t *testing.T) {
	t.Run("get working dir", func(t *testing.T) {
		wd, _ := os.Getwd()
		require.Equal(t, util.MustGetwd(), wd)
	})
}

func Test_FormatTime(t *testing.T) {
	t.Run("format time", func(t *testing.T) {
		tm := time.Date(2022, 2, 1, 2, 2, 2, 0, time.Now().Location())
		formatted := util.FormatTime(tm)
		require.Equal(t, "2022-02-01 02:02:02", formatted)

		parsed, err := util.ParseTime(formatted)
		require.NoError(t, err)
		require.Equal(t, tm, parsed)

		// Test empty time
		require.Equal(t, constants.TimeEmpty, util.FormatTime(time.Time{}))
		parsed, err = util.ParseTime(constants.TimeEmpty)
		require.NoError(t, err)
		require.Equal(t, time.Time{}, parsed)
	})
	t.Run("format time empty", func(t *testing.T) {
		// Test empty time
		require.Equal(t, constants.TimeEmpty, util.FormatTime(time.Time{}))
	})
}

func Test_ParseTime(t *testing.T) {
	t.Run("parse time", func(t *testing.T) {
		parsed, err := util.ParseTime("2022-02-01 02:02:02")
		require.NoError(t, err)
		require.Equal(t, time.Date(2022, 2, 1, 2, 2, 2, 0, time.Now().Location()), parsed)
	})
	t.Run("parse empty time", func(t *testing.T) {
		parsed, err := util.ParseTime(constants.TimeEmpty)
		require.NoError(t, err)
		require.Equal(t, time.Time{}, parsed)
	})
}

func Test_SplitCommand(t *testing.T) {
	t.Run("split command", func(t *testing.T) {
		cmd, args := util.SplitCommand("ls -al test/", false)
		require.Equal(t, "ls", cmd)

		require.Len(t, args, 2)
		require.Equal(t, "-al", args[0])
		require.Equal(t, "test/", args[1])
	})
	t.Run("split command with json", func(t *testing.T) {
		cmd, args := util.SplitCommand(`echo {\"key\":\"value\"}`, true)
		require.Equal(t, "echo", cmd)

		require.Len(t, args, 1)
		require.Equal(t, "{\"key\":\"value\"}", args[0])
	})
}

func Test_FileExits(t *testing.T) {
	t.Run("file exists", func(t *testing.T) {
		if !util.FileExists("/") {
			t.Fatal("file exists failed")
		}
	})
}

func Test_ValidFilename(t *testing.T) {
	t.Run("valid filename", func(t *testing.T) {
		ret := util.ValidFilename("file\\name", "_")
		require.Equal(t, ret, "file_name")
	})
}

func Test_OpenOrCreateFile(t *testing.T) {
	t.Run("open or create file", func(t *testing.T) {
		tmp, err := os.MkdirTemp("", "open_or_create")
		require.NoError(t, err)

		name := path.Join(tmp, "/file.txt")
		f, err := util.OpenOrCreateFile(name)
		require.NoError(t, err)

		defer func() {
			_ = f.Close()
			_ = os.Remove(name)
		}()

		if !util.FileExists(name) {
			t.Fatal("failed to create file")
		}
	})
	t.Run("open or create file and write", func(t *testing.T) {
		dir := util.MustTempDir("tempdir")
		defer func() {
			_ = os.RemoveAll(dir)
		}()

		filename := path.Join(dir, "test.txt")
		createdFile, err := util.OpenOrCreateFile(filename)
		require.NoError(t, err)
		defer func() {
			_ = createdFile.Close()
		}()

		_, err = createdFile.WriteString("test")
		require.NoError(t, err)
		require.NoError(t, createdFile.Sync(), err)
		require.NoError(t, createdFile.Close(), err)
		if !util.FileExists(filename) {
			t.Fatal("failed to create file")
		}

		openedFile, err := os.Open(filename)
		require.NoError(t, err)
		defer func() {
			_ = openedFile.Close()
		}()
		data, err := io.ReadAll(openedFile)
		require.NoError(t, err)
		require.Equal(t, "test", string(data))
	})
}

func Test_MustTempDir(t *testing.T) {
	t.Run("temp dir", func(t *testing.T) {
		dir := util.MustTempDir("tempdir")
		defer func() {
			_ = os.RemoveAll(dir)
		}()
		require.Contains(t, dir, "tempdir")
	})
}

func Test_LogErr(t *testing.T) {
	t.Run("log error", func(t *testing.T) {
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
	})
}

func TestTruncString(t *testing.T) {
	t.Run("trunc string", func(t *testing.T) {
		// Test empty string
		require.Equal(t, "", util.TruncString("", 8))
		// Test string with length less than limit
		require.Equal(t, "1234567", util.TruncString("1234567", 8))
		// Test string with length equal to limit
		require.Equal(t, "12345678", util.TruncString("123456789", 8))
	})
}

func TestMatchExtension(t *testing.T) {
	t.Run("match extension", func(t *testing.T) {
		// Test empty extension
		require.False(t, util.MatchExtension("test.txt", []string{}))
		// Test matching extension
		require.True(t, util.MatchExtension("test.txt", []string{".csv", ".txt"}))
		// Test matching extension
		require.False(t, util.MatchExtension("test.txt", []string{".csv"}))
	})
}
