# Crypto provider

Diia callback values are Base64 text wrapping encrypted or signed containers. The core module performs multipart parsing, size enforcement, Base64 decoding, metadata decoding, and package assembly. It does not redistribute native IIT/EUSign binaries.

Implement the vendor-neutral interface:

```go
type CryptoProvider interface {
    DecryptAndVerify(ctx context.Context, envelope []byte) ([]byte, error)
    Verify(ctx context.Context, signed []byte) ([]byte, error)
}
```

`DecryptAndVerify` must return authenticated plaintext from an encrypted and signed envelope. With the IIT API, a typical implementation calls `CtxDevelopData` followed by `VerifyDataInternalSpecific`.

`Verify` must verify a signed, unencrypted signature/hash part and return its verified content. It normally maps to the equivalent of `VerifyDataInternalSpecific`.

The IIT implementation is the standalone
[`go-diia-iit`](https://github.com/stremovskyy/go-diia-iit) module. It does not
import the core SDK. For Diia callbacks, install the optional
`github.com/stremovskyy/go-diia/iit` module, which joins the two packages and
exposes a provider accepted by `WithCryptoProvider`. Its
[callback example](../iit/examples/callback/main.go) shows the integration.
The IIT runtime uses a bring-your-own-official-archive installation and does
not redistribute IIT source or binaries.

## Local callback demo

Run `go run ./examples/callback` from the repository root. The example builds
synthetic multipart requests with `httptest`; it opens no listener, makes no
Diia API call, and needs no real credentials. It demonstrates `Parse`, `Process`,
`WriteSuccess`, and `WriteFailure` for a valid local request, a replay, and
an invalid bearer token. It prints only scenario names and HTTP status codes.

The example's `insecureDemoCrypto` strips marker strings from made-up bytes.
It performs **no encryption, decryption, or signature verification**. Its
in-memory pending-request map is also only a local replay demonstration. Neither
component can validate a real Diia callback.

## Real callback integration

A real callback handler needs a public HTTPS endpoint with TLS termination.
When Diia sends a callback bearer token, configure it with `WithCallbackToken`;
`Parse` rejects a present wrong token and rejects a missing token by default.
If the sandbox callback arrives
without an `Authorization` header, an application can explicitly set
`WithCallbackRequestIDAuthorizer` to check whether the trace ID is active in
its durable pending-request store. A present but invalid bearer token is
always rejected. This check does not consume the ID and does not replace the
application's atomic claim, cryptographic processing, and durable storage.
The missing-bearer mode can only be processed through the verified
`multi-sharing` ZIP path; separate-part callbacks still require Bearer.
Atomically claim the trace request ID in a durable, shared store before
processing. Keep the claim leased so a retriable failure can be retried; mark
the request complete only after verified output is durably stored, and handle
repeat deliveries without processing the same documents twice. Supply a real
`CryptoProvider`, such as the optional `github.com/stremovskyy/go-diia/iit`
provider configured with the required official runtime, certificates, and
partner key material.
Return success only after processing and securely handling the resulting
documents. Never use `insecureDemoCrypto` or the in-memory map in a deployed
handler. The sandbox acquirer credentials authorize outbound API calls; they do
not replace callback authentication against a bearer token or pending request,
or the cryptographic material.

For the legacy separate-part format, the SDK asks the provider to decrypt and
verify each envelope and returns verified `signature-*` bytes, but it does not
bind those bytes to a PDF. The partner must apply an audited
signature-to-document association policy and check document counts against
the pending request. `Parse` requires `encodeData`, but legacy `Process` does
not enforce completeness. Other unknown fields require an application
decision before acknowledgment.

For a `multi-sharing` ZIP callback, configure both `WithCryptoProvider` and
`WithMultisharingArchiveProcessor(diiaiit.NewMultisharingArchiveProcessor(provider))`.
`Process` verifies and decodes `encodeData`, obtains the ZIP password through
the authenticated `OfferRequestPassword` API, then delegates to the optional
IIT processor. The processor opens WinZip AES-256 entries in memory, checks
authentication and bounds, verifies the attached Diia seals with an exact-byte
comparison against metadata and PDFs, and verifies detached user signatures
against each exact PDF. Every
`metadata.data.<type>[i].fileName` must bind to one PDF and two signatures;
missing, duplicate, and extra entries fail. Two `vehicle-license` records
remain two `DocumentFile` values. The result includes the file-name links in
`DocumentFile` and `VerifiedSignature`; detached signatures do not duplicate
PDF bytes in `VerifiedSignature.Data`.
After processing, call `ValidateMultisharingPackage(package, mandatoryTypes...)`
with the mandatory types from the atomically claimed pending offer. The
validator checks the signed `documentTypes` manifest against verified PDFs,
allows a missing optional PDF only when `errorDocuments` records a supported
412, 404, or 500 result, and rejects every missing mandatory PDF.

Without an archive processor, `Process` rejects the ZIP callback. Configure
Diia encryption and signing certificates from a trusted source. Certificates
extracted from a callback can help diagnose format compatibility, but cannot
establish that the signer belongs to Diia. Native certificate status and
qualified-signature behavior need explicit validation against current sandbox
certificates and failure fixtures before relying on that claim.
If neither signed `encodeData` nor the signed ZIP metadata contains `requestId`,
configure `WithCallbackOfferRequestAuthorizer` to bind the trace ID to the
verified `offerRequestId` in the application's pending-request store.

## Processing pipeline

1. `CallbackService.Parse` validates the trace ID and, by default, the callback bearer token; the explicit missing-Bearer ZIP mode requires an active pending request ID.
2. The core reads multipart under a 256 MiB total limit, a 96 MiB encoded per-part limit, and a 1,024-part limit.
3. The core Base64-decodes `encodeData`, document fields, and `signature-*` fields.
4. For separate parts, `Process` calls `DecryptAndVerify` for metadata and each PDF envelope, then `Verify` for each signature/hash part.
5. For `multi-sharing`, `Process` verifies `encodeData`, obtains the archive password, and calls the configured archive processor.
6. The core retains original metadata JSON as `RawMetadata` and returns `DocumentPackage` only after the chosen processing path succeeds.

Diia documents the verified `signature-*` payload as a signed document hash,
but does not define a portable hash-to-PDF binding algorithm in this contract.
The SDK therefore returns the verified bytes unchanged. Applications must apply
their audited association policy instead of assuming or inventing an algorithm.

The provider owns native-library initialization, certificate stores, private-key contexts, trust policy, revocation/OCSP behavior, and safe shutdown. It must be concurrency-safe if a single SDK client is used concurrently. If the native vendor context is not concurrency-safe, serialize operations inside the adapter or maintain a bounded context pool.

## Distribution boundary

The `iit/euscp` package and platform archives are not part of the core
`go-diia` module. They are not available through the public Go module proxy,
and redistribution terms for the native binaries must be confirmed
independently. `go-diia-iit` keeps that legal and native lifecycle boundary in
its own Go module; `go-diia/iit` is an optional integration module.

## Security rules

- Never write plaintext metadata, PDFs, signed containers, callback bodies, or key material to logs.
- Never include native provider error buffers in public callback responses without sanitizing them.
- Atomically claim a pending request ID before invoking expensive crypto; use a lease or equivalent state to permit safe retries after failures.
- Commit completion after verified output is durably stored; reject or idempotently acknowledge duplicate deliveries without processing them again.
- Preserve `RawMetadata` for forward compatibility only in a protected data store with the same controls as the decoded personal data.
- Apply your retention and deletion policy to both typed metadata and decrypted files.
