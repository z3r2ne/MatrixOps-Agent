package memory

import "encoding/json"

func Clone(value *Memory) *Memory {
	if value == nil {
		return &Memory{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		copied := *value
		return &copied
	}
	var cloned Memory
	if err := json.Unmarshal(data, &cloned); err != nil {
		copied := *value
		return &copied
	}
	return &cloned
}
