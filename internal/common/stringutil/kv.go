package stringutil

import (
	"encoding/json"
	"strconv"
	"strings"
)

type KeyValue string

func NewKeyValue(key, value string) KeyValue {
	return KeyValue(key + "=" + value)
}

func (kv KeyValue) Key() string {
	parts := strings.SplitN(string(kv), "=", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func (kv KeyValue) Value() string {
	parts := strings.SplitN(string(kv), "=", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func (kv KeyValue) Bool() bool {
	v, err := strconv.ParseBool(kv.Value())
	if err != nil {
		return false
	}
	return v
}

func (kv KeyValue) String() string {
	return string(kv)
}

func (kv KeyValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(kv.String())
}

func (kv *KeyValue) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*kv = KeyValue(s)
	return nil
}
