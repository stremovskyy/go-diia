package metadata

import "encoding/json"

// MarshalJSON retains document types that are newer than this SDK when a
// decoded metadata value is edited and serialized again.
func (d MetadataData) MarshalJSON() ([]byte, error) {
	type plain MetadataData
	encoded, err := json.Marshal(plain(d))
	if err != nil {
		return nil, err
	}
	if len(d.Unknown) == 0 {
		return encoded, nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, err
	}
	for key, value := range d.Unknown {
		if _, known := fields[key]; !known {
			fields[key] = value
		}
	}
	return json.Marshal(fields)
}
