package models

import (
	"encoding/json"

	"github.com/yohamta/dagu/internal/filters"
)

type View struct {
	Name        string
	ContainTags []string
	Filter      filters.Filter
}

func ViewFromJson(s string) (*View, error) {
	v := &View{}
	err := json.Unmarshal([]byte(s), v)
	if err != nil {
		return nil, err
	}
	v.Filter = &filters.ContainTags{Tags: v.ContainTags}
	return v, err
}

func (v *View) ToJson() ([]byte, error) {
	js, err := json.Marshal(v)
	if err != nil {
		return []byte{}, err
	}
	return js, nil
}
