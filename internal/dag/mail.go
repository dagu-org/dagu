package dag

type SmtpConfig struct {
	Host string
	Port string
}

type MailConfig struct {
	From   string
	To     string
	Prefix string
}
