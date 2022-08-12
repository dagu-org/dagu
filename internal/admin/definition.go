package admin

type configDefinition struct {
	Host               string
	Port               int
	Env                map[string]string
	BaseConfig         string
	Dags               string
	Command            string
	WorkDir            string
	LogDir             string
	IsBasicAuth        bool
	BasicAuthUsername  string
	BasicAuthPassword  string
	LogEncodingCharset string
	NavbarColor        string
	NavbarTitle        string
}
