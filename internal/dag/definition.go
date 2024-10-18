// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
	HandlerOn         handlerOnDef
	Functions         []*funcDef
	Steps             []*stepDef
	SMTP              smtpConfigDef
	MailOn            *mailOnDef
	ErrorMail         mailConfigDef
	InfoMail          mailConfigDef
	TimeoutSec        int
	DelaySec          int
	RestartWaitSec    int
	HistRetentionDays *int
	Preconditions     []*conditionDef
	MaxActiveRuns     int
	Params            interface{}
	MaxCleanUpTimeSec *int
	Tags              any
}

type conditionDef struct {
	Condition string
	Expected  string
}

type handlerOnDef struct {
	Failure *stepDef
	Success *stepDef
	Cancel  *stepDef
	Exit    *stepDef
}

type stepDef struct {
	Name          string
	Description   string
	Dir           string
	Executor      any
	Command       any
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
	// Run is a sub workflow to run
	Run string
	// Params is a string of parameters to pass to the sub workflow
	Params string
}

type funcDef struct {
	Name    string
	Params  string
	Command string
}

type callFuncDef struct {
	Function string
	Args     map[string]any
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
