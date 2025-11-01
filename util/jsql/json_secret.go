package jsql

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
)

type Secret struct {
	sql.NullString
}

func SecretValue(s string) Secret {
	return Secret{
		NullString: sql.NullString{
			String: s,
			Valid:  true,
		},
	}
}

func SecretValueNull() Secret {
	return Secret{
		NullString: sql.NullString{
			Valid: false,
		},
	}
}

func (ns Secret) MarshalJSON() ([]byte, error) {
	return json.Marshal("**********")
}

func (ns *Secret) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		ns.Valid = false
		ns.String = ""
		return nil
	}
	ns.Valid = true
	return json.Unmarshal(b, &ns.String)
}

func (ns Secret) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return ns.String, nil
}

func (ns *Secret) Scan(src any) error {
	return ns.NullString.Scan(src)
}
