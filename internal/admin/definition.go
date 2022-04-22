package admin

type configDefinition struct {
	Host               string
	Port               int
	Env                map[string]string
	Jobs               string
	Command            string
	WorkDir            string
	IsBasicAuth        bool
	BasicAuthUsername  string
	BasicAuthPassword  string
	LogEncodingCharset string
}
