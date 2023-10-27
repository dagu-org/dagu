package dag

type configDefinition struct {
	Name              string
	Group             string
	Description       string
	Schedule          interface{}
	LogDir            string
	Env               interface{}
	HandlerOn         handerOnDef
	Functions         []*funcDef
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
	Executor      interface{}
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
	Env           string
	Call          *callFuncDef
}

type funcDef struct {
	Name    string
	Params  string
	Command string
}

type callFuncDef struct {
	Function string
	Args     map[string]interface{}
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
	Host     string
	Port     string
	Username string
	Password string
}

type mailConfigDef struct {
	From       string
	To         string
	Prefix     string
	AttachLogs bool
}

type mailOnDef struct {
	Failure bool
	Success bool
}
