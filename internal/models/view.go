package models

import (
	"encoding/json"
)

type View struct {
	Name        string
	Desc        string
	ContainTags []string
}

func ViewFromJson(b []byte) (*View, error) {
	v := &View{}
	err := json.Unmarshal(b, v)
	if err != nil {
		return nil, err
	}
	return v, err
}

func (v *View) ToJson() ([]byte, error) {
	js, err := json.Marshal(v)
	if err != nil {
		return []byte{}, err
	}
	return js, nil
}
