// Copyright 2025 The Dagu Authors
//
// Licensed under the GNU Affero General Public License v3.0.
// You may obtain a copy of the License at https://www.gnu.org/licenses/agpl-3.0.html

package core

import "fmt"

// CatchupPolicy determines what happens when scheduled DAG runs are missed
// during scheduler downtime.
type CatchupPolicy int

const (
	// CatchupPolicyOff is the default: no catch-up for missed runs.
	CatchupPolicyOff CatchupPolicy = iota
	// CatchupPolicyLatest runs only the latest missed interval.
	CatchupPolicyLatest
	// CatchupPolicyAll runs all missed intervals (up to cap).
	CatchupPolicyAll
)

// String returns the canonical lowercase token for the catchup policy.
func (c CatchupPolicy) String() string {
	switch c {
	case CatchupPolicyOff:
		return "off"
	case CatchupPolicyLatest:
		return "latest"
	case CatchupPolicyAll:
		return "all"
	default:
		return "off"
	}
}

// ParseCatchupPolicy parses a string into a CatchupPolicy.
func ParseCatchupPolicy(s string) (CatchupPolicy, error) {
	switch s {
	case "", "false", "off":
		return CatchupPolicyOff, nil
	case "latest":
		return CatchupPolicyLatest, nil
	case "all", "true":
		return CatchupPolicyAll, nil
	default:
		return CatchupPolicyOff, fmt.Errorf("unknown catchup policy: %q (expected false, off, latest, all, or true)", s)
	}
}
