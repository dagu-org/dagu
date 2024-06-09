package dag

// definition is a temporary struct to hold the DAG definition.
// This struct is used to unmarshal the YAML data.
// The data is then converted to the DAG struct.
type definition struct {
	Name              string
	Group             string
	Description       string
	Schedule          any
	LogDir            string
	Env               any
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
	Command       interface{}
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
	Run           string // Run is a sub workflow to run
	Params        string // Params is a string of parameters to pass to the sub workflow
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
