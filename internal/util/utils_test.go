// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package util_test

import (
	"bytes"
	"errors"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

func Test_MustGetUserHomeDir(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		err := os.Setenv("HOME", "/test")
		if err != nil {
			t.Fatal(err)
		}
		hd := fileutil.MustGetUserHomeDir()
		require.Equal(t, "/test", hd)
	})
}

func Test_MustGetwd(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		wd, _ := os.Getwd()
		require.Equal(t, fileutil.MustGetwd(), wd)
	})
}

func Test_FormatTime(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		tm := time.Date(2022, 2, 1, 2, 2, 2, 0, time.UTC)
		formatted := util.FormatTime(tm)
		require.Equal(t, "2022-02-01T02:02:02Z", formatted)

		parsed, err := util.ParseTime(formatted)
		require.NoError(t, err)
		require.Equal(t, tm, parsed)

		// Test empty time
		require.Equal(t, "-", util.FormatTime(time.Time{}))
		parsed, err = util.ParseTime("-")
		require.NoError(t, err)
		require.Equal(t, time.Time{}, parsed)
	})
	t.Run("Empty", func(t *testing.T) {
		// Test empty time
		require.Equal(t, "-", util.FormatTime(time.Time{}))
	})
}

func Test_ParseTime(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		parsed, err := util.ParseTime("2022-02-01T02:02:02Z")
		require.NoError(t, err)
		require.Equal(t, time.Date(2022, 2, 1, 2, 2, 2, 0, time.UTC), parsed)
	})
	t.Run("Valid_Legacy", func(t *testing.T) {
		parsed, err := util.ParseTime("2022-02-01 02:02:02")
		require.NoError(t, err)
		require.Equal(t, time.Date(2022, 2, 1, 2, 2, 2, 0, time.Now().Location()), parsed)
	})
	t.Run("Empty", func(t *testing.T) {
		parsed, err := util.ParseTime("-")
		require.NoError(t, err)
		require.Equal(t, time.Time{}, parsed)
	})
}

func Test_SplitCommand(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		cmd, args := util.SplitCommand("ls -al test/")
		require.Equal(t, "ls", cmd)
		require.Len(t, args, 2)
		require.Equal(t, "-al", args[0])
		require.Equal(t, "test/", args[1])
	})
	t.Run("WithJSON", func(t *testing.T) {
		cmd, args := util.SplitCommand(`echo {"key":"value"}`)
		require.Equal(t, "echo", cmd)
		require.Len(t, args, 1)
		require.Equal(t, `{"key":"value"}`, args[0])
	})
	t.Run("WithQuotedJSON", func(t *testing.T) {
		cmd, args := util.SplitCommand(`echo "{\"key\":\"value\"}"`)
		require.Equal(t, "echo", cmd)
		require.Len(t, args, 1)
		require.Equal(t, `"{\"key\":\"value\"}"`, args[0])
	})
}

func Test_SplitCommandWithParse(t *testing.T) {
	t.Run("CommandSubstitution", func(t *testing.T) {
		cmd, args := util.SplitCommandWithParse("echo `echo hello`")
		require.Equal(t, "echo", cmd)
		require.Len(t, args, 1)
		require.Equal(t, "hello", args[0])
	})
	t.Run("QuotedCommandSubstitution", func(t *testing.T) {
		cmd, args := util.SplitCommandWithParse("echo `echo \"hello world\"`")
		require.Equal(t, "echo", cmd)
		require.Len(t, args, 1)
		require.Equal(t, "hello world", args[0])
	})
	t.Run("EnvVar", func(t *testing.T) {
		os.Setenv("TEST_ARG", "hello")
		cmd, args := util.SplitCommandWithParse("echo $TEST_ARG")
		require.Equal(t, "echo", cmd)
		require.Len(t, args, 1)
		require.Equal(t, "hello", args[0])
	})
}

func Test_LogErr(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
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
	t.Run("Valid", func(t *testing.T) {
		// Test empty string
		require.Equal(t, "", util.TruncString("", 8))
		// Test string with length less than limit
		require.Equal(t, "1234567", util.TruncString("1234567", 8))
		// Test string with length equal to limit
		require.Equal(t, "12345678", util.TruncString("123456789", 8))
	})
}
