package admin

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var testLoadConfigYaml = `
dags: "` + "`echo /dags_dir`" + `"
host: localhost
port: 8081
command: /bin/current/dagu
workdir: /dags_dir
basicAuthUsername: user
basicAuthPassword: password
logEncodingCharset: utf-8
`

func TestLoadConfig(t *testing.T) {
	wd, _ := os.Getwd()
	for _, test := range []struct {
		Yaml string
		Want *Config
	}{
		{
			Yaml: testLoadConfigYaml,
			Want: &Config{
				DAGs:               "/dags_dir",
				Host:               "localhost",
				Port:               "8081",
				Command:            "/bin/current/dagu",
				WorkDir:            "/dags_dir",
				BasicAuthUsername:  "user",
				BasicAuthPassword:  "password",
				LogEncodingCharset: "utf-8",
				Env:                []string{},
			},
		},
		{
			Yaml: ``,
			Want: &Config{
				DAGs:               wd,
				Host:               "127.0.0.1",
				Port:               "8080",
				Command:            "dagu",
				WorkDir:            wd,
				BasicAuthUsername:  "",
				BasicAuthPassword:  "",
				LogEncodingCharset: "",
				Env:                []string{},
			},
		},
	} {
		l := &Loader{}
		d, err := l.unmarshalData([]byte(test.Yaml))
		require.NoError(t, err)

		def, err := l.decode(d)
		require.NoError(t, err)

		c, err := buildFromDefinition(def)
		require.NoError(t, err)

		c.setup()
		c.Env = []string{}

		require.Equal(t, test.Want, c)
	}
}

func TestLoadInvalidConfigError(t *testing.T) {
	for _, c := range []string{
		`dags: ./relative`,
		`dags: "` + "`ech /dags_dir`" + `"`,
		`command: "` + "`ech cmd`" + `"`,
		`workDir: "` + "`ech /dags`" + `"`,
		`basicAuthUsername: "` + "`ech foo`" + `"`,
		`basicAuthPassword: "` + "`ech foo`" + `"`,
		`logEncodingCharset: "` + "`ech foo`" + `"`,
	} {
		l := &Loader{}
		d, err := l.unmarshalData([]byte(c))
		require.NoError(t, err)

		def, err := l.decode(d)
		require.NoError(t, err)

		_, err = buildFromDefinition(def)
		require.Error(t, err)
	}
}
