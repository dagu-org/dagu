package config

type configDefinition struct {
	Name              string
	Description       string
	LogDir            string
	Env               map[string]string
	HandlerOn         handerOnDef
	Steps             []*stepDef
	Smtp              smtpConfigDef
	MailOn            mailOnDef
	ErrorMail         mailConfigDef
	InfoMail          mailConfigDef
	DelaySec          int
	HistRetentionDays *int
	Preconditions     []*conditionDef
	MaxActiveRuns     int
	Params            string
}

type conditionDef struct {
	Condition string
	Expected  string
}

type handerOnDef struct {
	Failure *stepDef
	Success *stepDef
	Cancel  *stepDef
	Exit    *stepDef
}

type stepDef struct {
	Name          string
	Description   string
	Dir           string
	Command       string
	Depends       []string
	ContinueOn    *continueOnDef
	RetryPolicy   *retryPolicyDef
	RepeatPolicy  *repeatPolicyDef
	MailOnError   bool
	Preconditions []*conditionDef
}

type continueOnDef struct {
	Failure bool
	Skipped bool
}

type repeatPolicyDef struct {
	Repeat      bool
	IntervalSec int
}

type retryPolicyDef struct {
	Limit int
}

type smtpConfigDef struct {
	Host string
	Port string
}

type mailConfigDef struct {
	From   string
	To     string
	Prefix string
}

type mailOnDef struct {
	Failure bool
	Success bool
}
