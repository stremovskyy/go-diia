# Contract and compatibility notes

This document records the SDK's wire-format choices and metadata compatibility
behavior. CI uses sanitized local fixtures.

## Deliberate wire-contract choices

- Offer deletion uses `/api/v1/acquirers/branch/{branch_id}/offer/{offer_id}`.
- Online sharing sends the actual `offerId`, not `branch_id`.
- Online status uses the `otp` query parameter.
- Temporary residence permits use `residence-permit-temporary`.
- Offline singleton metadata objects and online arrays normalize to slices.
- Numeric-or-quoted-numeric fields decode through `FlexibleNumber`. If a
  decoded `Metadata` has not been edited, marshaling preserves its original JSON
  shape, including numeric tokens, singleton objects, and unknown fields. After
  an edit, known document collections marshal as arrays and `FlexibleNumber`
  fields marshal as strings. Use `DocumentPackage.RawMetadata` when exact input
  bytes are required. Editing a typed document can also omit future fields
  nested inside that document; `MetadataData.Unknown` retains future document
  keys but not future fields within a known document type.

## Metadata compatibility choices

- The multi-sharing `errorDocuments` array contains objects with `documentType`
  and numeric `result`. `Metadata.ErrorDocumentDetails` exposes those entries as
  `[]ErrorDocument`. The existing `Metadata.ErrorDocuments` string view remains
  for callers of older SDK versions; changing it after decoding selects the
  legacy string representation for marshaling.
- The decoder accepts six alternate field spellings:
  `serieOriginal`/`serie Original`, `rnokpp`/`rnokpp?`,
  `powerKWT`/`powerKWT?`, `middleNameUA`/`middNameUA`,
  `issuedByUniversityNameEn`/`issueByUniversityNameEn`, and
  `specialityNameEn`/`specialtyNameEn`. The first spelling in each pair is used
  when encoding a modified model.
- Income-report employment and dismissal dates decode inside
  `SourcesOfIncome[].IncomeTaxMilitary`. Existing sibling fields on
  `IncomeSource` remain readable for compatibility. `IDrequest` is retained as
  an optional `IncomeReport.IDRequest`.
- Document types and defined value sets have named
  string types and constants. Incoming unknown values are retained so the
  decoder can read future Diia additions. Education `subType` and pension
  `pensionType` are not treated as closed enumerations.
- Tests use small sanitized payloads for supported field names.

## Forward compatibility

Unknown document keys are retained in `MetadataData.Unknown`, while
`DocumentPackage.RawMetadata` preserves the complete decrypted JSON envelope.
Known document-object fields remain typed; applications should use
`RawMetadata` when adopting newly published fields.

The SDK verifies and returns `signature-*` payload bytes. It does not bind
them to a decrypted PDF without an interoperable algorithm.
