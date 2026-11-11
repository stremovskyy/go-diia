# Usage

## Client lifecycle

Create one client and reuse it. Every service namespace shares its HTTP transport and concurrency-safe session token cache.

For commands that can be run in order with sandbox credentials, start with the
[examples walkthrough](../examples/README.md). The runnable examples default
to sandbox; the SDK constructor below targets production.

```go
client := diia.NewClient(
    os.Getenv("DIIA_ACQUIRER_TOKEN"),
    diia.WithTimeout(20*time.Second),
    diia.WithHTTPClient(http.DefaultClient),
)
```

Use both credentials for a sandbox client:

```go
client := diia.NewSandboxClient(
    os.Getenv("DIIA_ACQUIRER_TOKEN"),
    os.Getenv("DIIA_AUTH_ACQUIRER_TOKEN"),
)
```

`WithBaseURL` exists for contract tests and controlled gateways. Production applications should normally use the standard environment URLs selected by the constructors.

## Branches

```go
branch, err := client.Branches().Create().
    WithName("Kyiv office").
    WithEmail("sharing@example.com").
    WithRegion("Kyiv region").
    WithDistrict("Kyiv district").
    WithLocation("Kyiv").
    WithStreet("Khreshchatyk Street").
    WithHouse("1").
    WithCustomFullName("Example Partner — Kyiv").
    WithCustomFullAddress("1 Khreshchatyk Street, Kyiv").
    WithSharing(
        diia.DocumentTypeInternalPassport,
        diia.DocumentTypeTaxpayerCard,
    ).
    Do(ctx)
```

The builder always emits `deliveryTypes:["api"]` and `offerRequestType:"dynamic"`. Validation applies branch length limits before the first network call.

Use `Branches().List().WithSkip(0).WithLimit(100).Do(ctx)` or the equivalent offer builder. Omit either setter to omit that query parameter.

## Offers

```go
offer, err := client.Offers().Create().
    WithBranchID(branch.ID).
    WithName("Open an account").
    WithSharing(diia.DocumentTypeInternalPassport).
    WithReturnLink("https://partner.example/sharing/complete").
    Do(ctx)
```

Return links must be absolute HTTPS URLs.

## Offline sharing and status

```go
accepted, err := client.Sharing().RequestOffline().
    WithBranchID(branchID).
    WithBarcode(barcode).
    WithRequestID(requestID).
    WithDiiaID(true).
    Do(ctx)

status, err := client.Sharing().OfflineStatus().
    WithRequestID(requestID).
    Do(ctx)
```

The barcode must contain exactly 13 digits. `requestId` must contain 10–255 Unicode characters. Status lookup accepts barcode, request ID, or both.

## Online sharing and status

```go
link, err := client.Sharing().RequestOnline().
    WithBranchID(branchID).
    WithOfferID(offerID).
    WithRequestID(requestID).
    Do(ctx)
otp, err := link.OTP()

status, err := client.Sharing().OnlineStatus().WithOTP(otp).Do(ctx)
```

The known statuses are `initialized`, `processing`, `done`, `refuse`, and `notUsed`.

## Decrypted metadata

`DocumentPackage.Metadata` exposes document data as typed Go
models. For multi-sharing failures, inspect `Metadata.ErrorDocumentDetails`:

```go
for _, failed := range pkg.Metadata.ErrorDocumentDetails {
    fmt.Printf("%s: %d\n", failed.DocumentType, failed.Result)
}
```

The older `Metadata.ErrorDocuments` string view remains available. Prefer the
structured entries for new code. Public constants cover document types and
the supported gender, civil-record flag, IDP type, locality, and income
result values. The enum types retain unknown incoming strings so a newly
published value does not make metadata decoding fail.

`json.Marshal(pkg.Metadata)` preserves the JSON shape of unchanged metadata.
After editing typed fields, marshaling emits the canonical typed form and may
normalize number tokens or singleton documents. `pkg.RawMetadata` is the exact
decrypted JSON received from Diia. Editing a known typed document can drop
future fields that its Go model does not yet define.

## Dry run

```go
_, err := client.Offers().Create().
    WithBranchID(branchID).
    WithName("Open an account").
    WithSharing(diia.DocumentTypeInternalPassport).
    WithRunOptions(diia.DryRun()).
    Do(ctx)
```

An optional handler can inspect the typed payload. This is opt-in because branch and sharing request data can contain partner identifiers.

```go
option := diia.DryRun(func(endpoint string, payload any) {
    // Inspect only in a secure local environment.
})
```

## Errors

```go
branch, err := client.Branches().Get().WithBranchID(branchID).Do(ctx)
if errors.Is(err, diia.ErrNotFound) {
    // Handle a missing branch.
}

var apiErr *diia.APIError
if errors.As(err, &apiErr) {
    // apiErr has Name, Message, Code, Type, Data, HTTPStatus, and RawBody.
}
```

Transport, encoding, decoding, validation, callback, and crypto failures also expose sentinel errors. Do not log `RawBody` in production: an upstream error body is outside the SDK's redaction boundary.

## Callback failure messages

`WriteFailure` never echoes `err.Error()` by default:

```go
_ = client.Callbacks().WriteFailure().
    WithWriter(writer).
    WithError(err).
    Do(ctx)
// HTTP 404: {"success":false,"error":"callback processing failed"}
```

If Diia must receive a business-safe reason, configure an explicit mapper. A
blank result falls back to the generic text and output is capped at 512 Unicode
characters.

```go
client := diia.NewClient(token, diia.WithCallbackErrorMapper(func(err error) string {
    if errors.Is(err, context.DeadlineExceeded) {
        return "request timed out"
    }
    return ""
}))
```

## Request models

Public payload models remain available from the root package with zero-argument fluent constructors. They use the same normalization and defaults as executable builders:

```go
payload := diia.NewOnlineShareRequest().
    WithBranchID(branchID).
    WithOfferID(offerID).
    WithRequestID(requestID)
```

Payload models do not perform I/O and therefore do not expose `Do`. Use the namespace builders for execution. A builder may be executed more than once; every `Do(ctx)` is a new operation.
