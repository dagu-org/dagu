package fileutil

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_MustGetUserHomeDir(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		err := os.Setenv("HOME", "/test")
		if err != nil {
			t.Fatal(err)
		}
		hd := MustGetUserHomeDir()
		require.Equal(t, "/test", hd)
	})
}

func Test_MustGetwd(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		wd, _ := os.Getwd()
		require.Equal(t, MustGetwd(), wd)
	})
}

func Test_FileExits(t *testing.T) {
	t.Run("Exists", func(t *testing.T) {
		if !FileExists("/") {
			t.Fatal("file exists failed")
		}
	})
}

func Test_OpenOrCreateFile(t *testing.T) {
	t.Run("OpenOrCreate", func(t *testing.T) {
		tmp, err := os.MkdirTemp("", "open_or_create")
		require.NoError(t, err)

		name := filepath.Join(tmp, "/file.txt")
		f, err := OpenOrCreateFile(name)
		require.NoError(t, err)

		defer func() {
			_ = f.Close()
			_ = os.Remove(name)
		}()

		if !FileExists(name) {
			t.Fatal("failed to create file")
		}
	})
	t.Run("OpenOrCreateThenWrite", func(t *testing.T) {
		dir := MustTempDir("tempdir")
		defer func() {
			_ = os.RemoveAll(dir)
		}()

		filename := filepath.Join(dir, "test.txt")
		createdFile, err := OpenOrCreateFile(filename)
		require.NoError(t, err)
		defer func() {
			_ = createdFile.Close()
		}()

		_, err = createdFile.WriteString("test")
		require.NoError(t, err)
		require.NoError(t, createdFile.Sync(), err)
		require.NoError(t, createdFile.Close(), err)
		if !FileExists(filename) {
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
	t.Run("Valid", func(t *testing.T) {
		dir := MustTempDir("tempdir")
		defer func() {
			_ = os.RemoveAll(dir)
		}()
		require.Contains(t, dir, "tempdir")
	})
}

func TestTruncString(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		// Test empty string
		require.Equal(t, "", TruncString("", 8))
		// Test string with length less than limit
		require.Equal(t, "1234567", TruncString("1234567", 8))
		// Test string with length equal to limit
		require.Equal(t, "12345678", TruncString("123456789", 8))
	})
}

func TestIsYAMLFile(t *testing.T) {
	tests := []struct {
		file string
		want bool
	}{
		{"config.yaml", true},
		{"config.yml", true},
		{"config.json", false},
		{"config", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := IsYAMLFile(tt.file); got != tt.want {
			t.Errorf("IsYAMLFile(%q) = %v, want %v", tt.file, got, tt.want)
		}
	}
}

func TestAddYamlExtension(t *testing.T) {
	tests := []struct {
		file string
		want string
	}{
		{"config", "config.yaml"},
		{"config.yml", "config.yml"},
		{"config.yaml", "config.yaml"},
		{"config.json", "config.json.yaml"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := EnsureYAMLExtension(tt.file); got != tt.want {
			t.Errorf("AddYamlExtension(%q) = %q, want %q", tt.file, got, tt.want)
		}
	}
}
