// Copyright 2025 The Dagu Authors
//
// Licensed under the GNU Affero General Public License v3.0.
// You may obtain a copy of the License at https://www.gnu.org/licenses/agpl-3.0.html

package core

import "fmt"

// MisfirePolicy determines what happens when scheduled DAG runs are missed
// during scheduler downtime.
type MisfirePolicy int

const (
	// MisfirePolicyIgnore is the default: no catch-up for missed runs.
	MisfirePolicyIgnore MisfirePolicy = iota
	// MisfirePolicyRunOnce runs only the earliest missed interval.
	MisfirePolicyRunOnce
	// MisfirePolicyRunLatest runs only the latest missed interval.
	MisfirePolicyRunLatest
	// MisfirePolicyRunAll runs all missed intervals (up to cap).
	MisfirePolicyRunAll
)

// String returns the canonical lowercase token for the misfire policy.
func (m MisfirePolicy) String() string {
	switch m {
	case MisfirePolicyIgnore:
		return "ignore"
	case MisfirePolicyRunOnce:
		return "runOnce"
	case MisfirePolicyRunLatest:
		return "runLatest"
	case MisfirePolicyRunAll:
		return "runAll"
	default:
		return "ignore"
	}
}

// ParseMisfirePolicy parses a string into a MisfirePolicy.
func ParseMisfirePolicy(s string) (MisfirePolicy, error) {
	switch s {
	case "", "ignore":
		return MisfirePolicyIgnore, nil
	case "runOnce":
		return MisfirePolicyRunOnce, nil
	case "runLatest":
		return MisfirePolicyRunLatest, nil
	case "runAll":
		return MisfirePolicyRunAll, nil
	default:
		return MisfirePolicyIgnore, fmt.Errorf("unknown misfire policy: %q (expected ignore, runOnce, runLatest, or runAll)", s)
	}
}
