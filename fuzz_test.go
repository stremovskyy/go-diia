package go_diia

import (
	"encoding/json"
	"testing"
)

func FuzzDecodeCallbackBase64(f *testing.F) {
	for _, seed := range []string{"cGF5bG9hZA==", "cGF5bG9hZA", "YQ==\n", "%%%", ""} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, value string) {
		_, _ = decodeCallbackBase64([]byte(value))
	})
}

func FuzzDecodeAPIError(f *testing.F) {
	for _, seed := range []string{
		`{"message":"bad"}`,
		`{"error":{"name":"Error","message":"nested"}}`,
		`{"error":"missing"}`,
		`not-json`,
		``,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, body string) {
		err := decodeAPIError(400, []byte(body))
		_ = err.Error()
		_ = errorCategory(err)
	})
}

func FuzzMetadataDecode(f *testing.F) {
	for _, seed := range []string{
		`{"data":{}}`,
		`{"errorDocuments":"foreign-passport","data":{"foreign-passport":{}}}`,
		`{"data":{"education-document":[{"subType":"master"}]}}`,
		`null`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, wire string) {
		var metadata Metadata

		if json.Unmarshal([]byte(wire), &metadata) == nil {
			_, _ = json.Marshal(metadata)
		}
	})
}
