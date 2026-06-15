package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// UintSlice stores JSON arrays of uint IDs and tolerates legacy single-number values.
type UintSlice []uint

func (s UintSlice) Slice() []uint {
	return []uint(s)
}

func normalizeUintSliceJSON(data []byte) ([]uint, error) {
	trimmed := string(data)
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	var single uint
	if err := json.Unmarshal(data, &single); err == nil {
		return []uint{single}, nil
	}
	var many []uint
	if err := json.Unmarshal(data, &many); err != nil {
		return nil, err
	}
	return many, nil
}

func (s *UintSlice) UnmarshalJSON(data []byte) error {
	ids, err := normalizeUintSliceJSON(data)
	if err != nil {
		return err
	}
	*s = UintSlice(ids)
	return nil
}

func (s UintSlice) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("null"), nil
	}
	return json.Marshal([]uint(s))
}

func (s *UintSlice) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("UintSlice: unsupported type %T", value)
	}
	ids, err := normalizeUintSliceJSON(data)
	if err != nil {
		return err
	}
	*s = UintSlice(ids)
	return nil
}

func (s UintSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal([]uint(s))
}
