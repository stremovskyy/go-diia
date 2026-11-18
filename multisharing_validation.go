package go_diia

// ValidateMultisharingPackage checks that a verified multi-sharing package
// contains a PDF for every mandatory document type from the pending offer.
// It also checks the signed metadata's documentTypes and errorDocuments
// against the PDFs returned by the archive processor. Diia may omit a failed
// optional type from documentTypes while recording it in errorDocuments. The
// caller must check every PDF and error type against the pending offer. Call this
// after Process and before storing documents or acknowledging the callback.
func ValidateMultisharingPackage(pkg *DocumentPackage, mandatory ...DocumentType) error {
	if pkg == nil || pkg.Metadata == nil || pkg.MetadataSealFileName == "" || len(pkg.RawMetadata) == 0 ||
		len(pkg.Documents) == 0 {
		return callbackError("verified multi-sharing package is required", nil)
	}

	declared := make(map[DocumentType]bool, len(pkg.Metadata.DocumentTypes))

	for _, documentType := range pkg.Metadata.DocumentTypes {
		if !documentType.Valid() || declared[documentType] {
			return callbackError("multi-sharing metadata has an invalid or duplicate document type", nil)
		}

		declared[documentType] = true
	}

	if len(declared) == 0 {
		return callbackError("multi-sharing metadata has no document types", nil)
	}

	actual := make(map[DocumentType]bool, len(pkg.Documents))

	for _, document := range pkg.Documents {
		if !document.Type.Valid() || len(document.PDF) == 0 || !declared[document.Type] {
			return callbackError("multi-sharing PDF does not match metadata document types", nil)
		}

		actual[document.Type] = true
	}

	// Typed errorDocuments use result 412 (unchecked), 404 (missing), or 500
	// (registry failure).
	// Legacy string errors cannot be interpreted safely as these objects.
	if len(pkg.Metadata.ErrorDocuments) != len(pkg.Metadata.ErrorDocumentDetails) {
		return callbackError("multi-sharing metadata has an unrecognized document error", nil)
	}

	failed := make(map[DocumentType]bool, len(pkg.Metadata.ErrorDocumentDetails))

	for _, document := range pkg.Metadata.ErrorDocumentDetails {
		if !document.DocumentType.Valid() {
			return callbackError("multi-sharing metadata has an unsupported document error type", nil)
		}
		if actual[document.DocumentType] {
			return callbackError("multi-sharing metadata document error conflicts with PDF", nil)
		}
		if failed[document.DocumentType] {
			return callbackError("multi-sharing metadata has duplicate document error type", nil)
		}
		if document.Result != 412 && document.Result != 404 && document.Result != 500 {
			return callbackError("multi-sharing metadata has invalid document error result", nil)
		}

		failed[document.DocumentType] = true
	}

	for documentType := range declared {
		if !actual[documentType] && !failed[documentType] {
			return callbackError("multi-sharing metadata declares a document without a PDF or error", nil)
		}
	}

	for _, documentType := range mandatory {
		if !documentType.Valid() || !actual[documentType] {
			return callbackError("multi-sharing mandatory document is missing", nil)
		}
	}

	return nil
}
