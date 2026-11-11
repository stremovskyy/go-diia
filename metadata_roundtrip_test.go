package go_diia

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMetadataUnchangedMarshalRetainsCatalogWireShapes(t *testing.T) {
	const wire = `{"errorDocuments":[{"documentType":"internal-passport","result":404}],"data":{"internal-passport":{"docNumber":"A","futureField":42},"future-document":{"code":1}},"futureEnvelopeField":true}`
	var metadata Metadata
	if err := json.Unmarshal([]byte(wire), &metadata); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	var original, roundTrip any
	if err := json.Unmarshal([]byte(wire), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundTrip, original) {
		t.Fatalf("unchanged metadata changed its JSON shape: %s", encoded)
	}
}

func TestMetadataEmptyErrorsDoNotChangeNumericOrSingletonWireShape(t *testing.T) {
	const wire = `{"errorDocuments":[],"data":{"foreign-passport":{"registration":{"country_id":804}}}}`
	var metadata Metadata
	if err := json.Unmarshal([]byte(wire), &metadata); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	var original, roundTrip any
	if err := json.Unmarshal([]byte(wire), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundTrip, original) {
		t.Fatalf("empty errors changed numeric or singleton wire shape: %s", encoded)
	}
}

func TestMetadataUnknownDocumentMutationIsSerialized(t *testing.T) {
	const wire = `{"errorDocuments":[],"data":{"future-document":{"code":1}}}`
	var metadata Metadata
	if err := json.Unmarshal([]byte(wire), &metadata); err != nil {
		t.Fatal(err)
	}
	metadata.Data.Unknown["future-document"] = json.RawMessage(`{"code":2}`)
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		Data map[string]struct {
			Code int `json:"code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatal(err)
	}
	if output.Data["future-document"].Code != 2 {
		t.Fatalf("unknown document mutation was not serialized: %s", encoded)
	}
}

func TestMetadataLegacyErrorViewMutationIsSerialized(t *testing.T) {
	const wire = `{"errorDocuments":[{"documentType":"internal-passport","result":404}],"data":{}}`
	var metadata Metadata
	if err := json.Unmarshal([]byte(wire), &metadata); err != nil {
		t.Fatal(err)
	}
	metadata.ErrorDocuments[0] = "taxpayer-card"
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		ErrorDocuments []string `json:"errorDocuments"`
	}
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatalf("legacy error view did not take precedence: %v", err)
	}
	if len(output.ErrorDocuments) != 1 || output.ErrorDocuments[0] != "taxpayer-card" {
		t.Fatalf("legacy error mutation was not serialized: %s", encoded)
	}
}

func TestMetadataClearedStructuredErrorsMarshalAsEmptyArray(t *testing.T) {
	const wire = `{"errorDocuments":[{"documentType":"internal-passport","result":404}],"data":{}}`
	var metadata Metadata
	if err := json.Unmarshal([]byte(wire), &metadata); err != nil {
		t.Fatal(err)
	}
	metadata.ErrorDocumentDetails = nil
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		ErrorDocuments json.RawMessage `json:"errorDocuments"`
	}
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatal(err)
	}
	if string(output.ErrorDocuments) != "[]" {
		t.Fatalf("cleared structured errors became legacy strings: %s", encoded)
	}
}
