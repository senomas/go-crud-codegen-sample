package jsql

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
)

type NullString struct {
	sql.NullString
}

func NullStringValue(s string) NullString {
	return NullString{
		NullString: sql.NullString{
			String: s,
			Valid:  true,
		},
	}
}

func NullStringValueNull() NullString {
	return NullString{
		NullString: sql.NullString{
			Valid: false,
		},
	}
}

func (ns NullString) MarshalJSON() ([]byte, error) {
	if !ns.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(ns.String)
}

func (ns *NullString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		ns.Valid = false
		ns.String = ""
		return nil
	}
	ns.Valid = true
	return json.Unmarshal(b, &ns.String)
}

func (ns NullString) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return ns.String, nil
}

func (ns *NullString) Scan(src any) error {
	return ns.NullString.Scan(src)
}

type NullInt64 struct {
	sql.NullInt64
}

func NullInt64Value(i int64) NullInt64 {
	return NullInt64{
		NullInt64: sql.NullInt64{
			Int64: i,
			Valid: true,
		},
	}
}

func NullInt64ValueNull() NullInt64 {
	return NullInt64{
		NullInt64: sql.NullInt64{
			Valid: false,
		},
	}
}

func (n NullInt64) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Int64)
}

func (n *NullInt64) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		n.Valid = false
		n.Int64 = 0
		return nil
	}
	n.Valid = true
	return json.Unmarshal(b, &n.Int64)
}

func (n NullInt64) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Int64, nil
}

func (n *NullInt64) Scan(src any) error {
	return n.NullInt64.Scan(src)
}
