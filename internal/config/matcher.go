package config

// Matcher is a function that returns true
// if the given config matches the filter.
type Matcher interface {
	Matches(cfg *Config) bool
}

// ContainTagsMatcher checks if the config contains
// all the tags.
type ContainTagsMatcher struct {
	Tags []string
}

var _ Matcher = (*ContainTagsMatcher)(nil)

func (ct *ContainTagsMatcher) Matches(cfg *Config) bool {
	for _, tag := range ct.Tags {
		if !cfg.HasTag(tag) {
			return false
		}
	}
	return true
}
