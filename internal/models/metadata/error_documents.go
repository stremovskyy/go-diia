package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

// ErrorDocument is one failed document in a multi-sharing metadata response.
type ErrorDocument struct {
	DocumentType DocumentType `json:"documentType"`
	Result       int          `json:"result"`
}

// UnmarshalJSON keeps the historical ErrorDocuments string view while exposing
// the structured errorDocuments entries published for multi-sharing.
func (m *Metadata) UnmarshalJSON(data []byte) error {
	type plain Metadata
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	var wire struct {
		ErrorDocuments json.RawMessage `json:"errorDocuments"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	var entries []json.RawMessage
	value := bytes.TrimSpace(wire.ErrorDocuments)
	if len(value) > 0 && !bytes.Equal(value, []byte("null")) {
		if value[0] == '[' {
			if err := json.Unmarshal(value, &entries); err != nil {
				return fmt.Errorf("errorDocuments: %w", err)
			}
		} else {
			entries = []json.RawMessage{value}
		}
	}

	*m = Metadata(decoded)
	for _, entry := range entries {
		entry = bytes.TrimSpace(entry)
		if len(entry) == 0 || entry[0] != '{' {
			continue
		}

		var document ErrorDocument
		if err := json.Unmarshal(entry, &document); err != nil {
			return fmt.Errorf("errorDocuments entry: %w", err)
		}
		m.ErrorDocumentDetails = append(m.ErrorDocumentDetails, document)
	}
	var err error
	if m.ErrorDocuments != nil {
		m.originalLegacyErrors = append(StringList{}, m.ErrorDocuments...)
	}
	if m.ErrorDocumentDetails != nil {
		m.originalErrorDetails = append([]ErrorDocument{}, m.ErrorDocumentDetails...)
	}
	m.normalizedJSON, err = marshalCanonicalMetadata(*m, false, false)
	if err != nil {
		return err
	}
	m.rawJSON = append(json.RawMessage(nil), data...)
	return nil
}

// MarshalJSON preserves the JSON shape of an unchanged input envelope. A
// modified model uses the canonical typed representation, including errors.
func (m Metadata) MarshalJSON() ([]byte, error) {
	legacyChanged := !reflect.DeepEqual(m.ErrorDocuments, m.originalLegacyErrors)
	detailsChanged := !reflect.DeepEqual(m.ErrorDocumentDetails, m.originalErrorDetails)
	preferLegacy := len(m.rawJSON) != 0 && legacyChanged && !detailsChanged
	encoded, err := marshalCanonicalMetadata(m, preferLegacy, detailsChanged)
	if err != nil {
		return nil, err
	}
	if len(m.rawJSON) != 0 && !legacyChanged && !detailsChanged && bytes.Equal(encoded, m.normalizedJSON) {
		return append([]byte(nil), m.rawJSON...), nil
	}
	return encoded, nil
}

func marshalCanonicalMetadata(m Metadata, preferLegacy, forceTyped bool) ([]byte, error) {
	type plain Metadata
	var errors any = []ErrorDocument{}
	if !preferLegacy && len(m.ErrorDocumentDetails) > 0 {
		errors = m.ErrorDocumentDetails
	} else if !forceTyped && m.ErrorDocuments != nil {
		errors = m.ErrorDocuments
	}
	return json.Marshal(struct {
		*plain
		ErrorDocuments any `json:"errorDocuments"`
	}{plain: (*plain)(&m), ErrorDocuments: errors})
}
