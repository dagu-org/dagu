package dag

type configDefinition struct {
	Name              string
	Group             string
	Description       string
	Schedule          interface{}
	LogDir            string
	Env               interface{}
	HandlerOn         handerOnDef
	Steps             []*stepDef
	Smtp              smtpConfigDef
	MailOn            *mailOnDef
	ErrorMail         mailConfigDef
	InfoMail          mailConfigDef
	DelaySec          int
	RestartWaitSec    int
	HistRetentionDays *int
	Preconditions     []*conditionDef
	MaxActiveRuns     int
	Params            string
	MaxCleanUpTimeSec *int
	Tags              string
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
	Executor      string
	Command       string
	Script        string
	Stdout        string
	Stderr        string
	Output        string
	Depends       []string
	ContinueOn    *continueOnDef
	RetryPolicy   *retryPolicyDef
	RepeatPolicy  *repeatPolicyDef
	MailOnError   bool
	Preconditions []*conditionDef
	SignalOnStop  *string
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
	Limit       int
	IntervalSec int
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
