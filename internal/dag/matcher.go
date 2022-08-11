package dag

// Matcher is a function that returns true
// if the given config matches the filter.
type Matcher interface {
	Matches(d *DAG) bool
}

// ContainTagsMatcher checks if the config contains
// all the tags.
type ContainTagsMatcher struct {
	Tags []string
}

var _ Matcher = (*ContainTagsMatcher)(nil)

func (ct *ContainTagsMatcher) Matches(d *DAG) bool {
	for _, tag := range ct.Tags {
		if !d.HasTag(tag) {
			return false
		}
	}
	return true
}
