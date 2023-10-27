package dag

type SmtpConfig struct {
	Host     string
	Port     string
	Username string
	Password string
}

type MailConfig struct {
	From       string
	To         string
	Prefix     string
	AttachLogs bool
}
