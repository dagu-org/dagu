package filters

import "github.com/yohamta/dagu/internal/config"

type Filter interface {
	Matches(config *config.Config) bool
}

type ContainTags struct {
	Tags []string
}

var _ Filter = (*ContainTags)(nil)

func (ct *ContainTags) Matches(cfg *config.Config) bool {
	for _, tag := range ct.Tags {
		if !cfg.HasTag(tag) {
			return false
		}
	}
	return true
}
