package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// FlexibleNumber accepts both a JSON number and a quoted JSON number. It
// marshals deterministically as a quoted string.
type FlexibleNumber string

func (n *FlexibleNumber) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)

	if string(data) == "null" {
		*n = ""

		return nil
	}

	var quoted string

	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &quoted); err != nil {
			return err
		}

		*n = FlexibleNumber(quoted)

		return nil
	}

	var number json.Number

	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("flexible number: %w", err)
	}

	*n = FlexibleNumber(number.String())

	return nil
}

func (n FlexibleNumber) MarshalJSON() ([]byte, error) { return json.Marshal(string(n)) }

// StringList tolerates a single string, a list, null, and mixed primitive
// values in errorDocuments.
type StringList []string

func (s *StringList) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)

	if string(data) == "null" || len(data) == 0 {
		*s = nil

		return nil
	}

	var values []any
	if data[0] == '[' {
		if err := json.Unmarshal(data, &values); err != nil {
			return err
		}
	} else {
		var value any

		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}

		values = []any{value}
	}
	result := make([]string, 0, len(values))

	for _, value := range values {
		switch typed := value.(type) {
		case string:
			result = append(result, typed)
		case nil:
			continue

		default:
			encoded, _ := json.Marshal(typed)
			result = append(result, string(encoded))
		}
	}

	*s = result

	return nil
}
