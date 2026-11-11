# go-diia

`go-diia` is a public Go SDK for the Diia Document Sharing API. It follows the same small-interface, constructor, and fluent chain style as [`go-monobank`](https://github.com/stremovskyy/go-monobank), [`go-ipay`](https://github.com/stremovskyy/go-ipay), and [`go-platon`](https://github.com/stremovskyy/go-platon). Request builders execute through `Do(ctx)`; `Sharing().OfferRequestPassword(ctx, id)` is a direct read operation.

The module covers offline sharing, online/deeplink sharing, branches, offers, request statuses, and encrypted multipart callbacks. The optional IIT module verifies multi-sharing ZIP signatures and binds documents to their metadata.

All public request, response, callback, metadata, enum, and value types remain available directly from the root `go_diia` package. Their implementations are grouped internally by API, domain, callback, and metadata concerns, so SDK users do not need additional model imports.

## Requirements

- Go 1.26 or newer to compile. Use a currently supported, patched Go release
  in production (Go 1.26.10 or newer as of November 2026).
- Diia `acquirer_token`
- Sandbox `auth_acquirer_token` when using the sandbox
- A `CryptoProvider` implementation when processing callbacks. Integration with
  the official IIT runtime is available through the optional `github.com/stremovskyy/go-diia/iit` module.

## Install

```bash
go get github.com/stremovskyy/go-diia
```

For the optional IIT callback provider, install `github.com/stremovskyy/go-diia/iit`
separately and follow its [native runtime setup](iit/README.md). The core SDK
does not require IIT.

The root CI runs on pushes and pull requests without IIT. Native IIT checks
run in a [separate manual workflow](iit/README.md) with officially supplied
archives.

## Client

Production is the default:

```go
client := diia.NewDefaultClient(os.Getenv("DIIA_ACQUIRER_TOKEN"))
```

Use the dedicated sandbox constructor for test credentials:

```go
client := diia.NewSandboxClient(
    os.Getenv("DIIA_ACQUIRER_TOKEN"),
    os.Getenv("DIIA_AUTH_ACQUIRER_TOKEN"),
)
```

The session token is fetched lazily, cached safely across goroutines for its
two-hour lifetime, and refreshed five minutes early. An API `401`
invalidates the cached token. The SDK retries a `GET` or `HEAD` once after
refreshing it; requests that change state are not replayed. The sandbox Basic
credential is sent only to the sandbox auth endpoint. It is never added to
production requests.

## Runnable examples

Start with the [sandbox walkthrough](examples/README.md) for the exact
environment variables and command order. The examples cover authentication,
branch and offer management, online and offline sharing requests, and status
checks. Their default environment is sandbox; production requires an explicit
selection and a separate opt-in for operations that change state. The
[callback example](examples/callback/main.go) is a local demonstration with
synthetic data, not a live callback server.

## Offline sharing

```go
response, err := client.Sharing().RequestOffline().
    WithBranchID(branchID).
    WithBarcode(barcode).
    WithRequestID(requestID).
    WithDiiaID(true).
    Do(ctx)
```

## Online sharing

```go
deeplink, err := client.Sharing().RequestOnline().
    WithBranchID(branchID).
    WithOfferID(offerID).
    WithRequestID(requestID).
    WithReturnLink("https://partner.example/sharing/complete").
    WithDiiaID(true).
    Do(ctx)
if err == nil {
    otp, err := deeplink.OTP()
    _ = otp
    _ = err
}
```

Multi-sharing offers can mark document types that the user must select. Every
`mandatorySharing` type must also appear in `sharing`; this does not set how
many documents of a type the user chooses. The field is omitted unless set:

```go
offer, err := client.Offers().Create().
    WithBranchID(branchID).
    WithName("Document package").
    WithSharing(diia.DocumentTypeInternalPassport, diia.DocumentTypeVehicleLicense).
    WithMandatorySharing(diia.DocumentTypeInternalPassport).
    Do(ctx)
```

For an online request that asks Diia to sign with `DSTU4145-DSTU7564`, add
`WithSignAlgo("DSTU4145-DSTU7564")`. It is omitted by default. Once the
verified callback yields an `offerRequestId`, retrieve the archive password
with `client.Sharing().OfferRequestPassword(ctx, offerRequestID)`; the SDK uses
the current session token and refreshes it on a GET `401`. Keep the password
out of logs and persistent records.

The SDK uses these wire contracts:

- offer deletion uses `/api/v1/acquirers/branch/{branch_id}/offer/{offer_id}`;
- online request JSON sends the actual `offerId`, not `branch_id`;
- online status sends the query key `otp`, not `barcode`;
- temporary residence permits use `residence-permit-temporary`.

## Callback security

Configure `DIIA_CALLBACK_TOKEN` when Diia sends a callback bearer token.
`Parse` rejects a present wrong token and, by default, a missing token. For
sandbox ZIP callbacks without `Authorization`, use the explicit pending-request
authorization mode described below.

```go
client := diia.NewClient(
    os.Getenv("DIIA_ACQUIRER_TOKEN"),
    diia.WithCallbackToken(os.Getenv("DIIA_CALLBACK_TOKEN")),
    diia.WithCryptoProvider(provider),
)
```

`Callbacks().Parse().WithRequest(request).Do(ctx)` validates the configured bearer token, reads multipart fields with size limits, Base64-decodes their envelopes, preserves repeated document types, and normalizes names such as `foreign-passport[]`. `Process().WithPayload(payload).Do(ctx)` handles the older separate-part format or delegates `multi-sharing` ZIP processing to the optional IIT processor. It returns typed metadata, authenticated PDFs, signature links, and the original `RawMetadata` JSON.

For an observed sandbox callback with no `Authorization` header, explicitly set `WithCallbackRequestIDAuthorizer` to check an active pending request. A present malformed or wrong bearer always fails. This mode requires the verified ZIP processing path. If signed data omits `requestId`, set `WithCallbackOfferRequestAuthorizer` to bind the trace ID to the signed `offerRequestId`.

After `Process` returns a ZIP package, call `ValidateMultisharingPackage` with
the mandatory document types stored for that pending offer. It rejects a
missing mandatory PDF and inconsistencies between signed `documentTypes`,
document errors, and verified PDFs. Optional document errors remain available
in `Metadata.ErrorDocumentDetails`.

The optional `github.com/stremovskyy/go-diia/iit` module supplies the official
IIT provider. Its [callback example](iit/examples/callback/main.go) shows how to
configure it and pass it to `WithCryptoProvider` and `WithMultisharingArchiveProcessor`.

Before processing, **atomically claim the `X-Document-Request-Trace-Id` against a pending request in your durable store**. Mark it complete only after verified documents are stored; handle retries idempotently and release or expire claims after retriable failures. Header possession alone is not proof that the callback belongs to an outstanding partner request.

The callback endpoint must follow Diia's current TLS, public accessibility,
keepalive, and response-time requirements. A success response is HTTP 200 with
`{"success":true}`. A failure response is HTTP 404 and returns the generic
message `callback processing failed` by default. Use
`WithCallbackErrorMapper` only when the application can map an internal error
to a deliberately sanitized public message.

See the [local callback demonstration](examples/callback/main.go) and
[crypto integration requirements](docs/Crypto.md).

## Production safety

- Request builders terminate in `Do(context.Context)`; a nil context is rejected. `OfferRequestPassword` is a direct read method with a context.
- Builders are mutable configuration objects and are not goroutine-safe.
- Clients and namespace services are safe for concurrent use. A custom
  `CryptoProvider` must provide its own concurrency safety; the optional IIT
  provider serializes native calls.
- The default request timeout is 30 seconds and can be changed with `WithTimeout`.
- `DryRun` validates and builds mutating requests without sending them.
- `Recorder` receives method, sanitized path, status, duration, and a coarse error category only.
- Logs and recorder records never include tokens, HTTP bodies, callback bodies, metadata, PDFs, request query values, or personal data.
- API failures support `errors.Is` and `errors.As` through sentinels and `*APIError`.
- Callback and crypto failures preserve their typed causes for `errors.Is/As`
  without exposing those causes in HTTP responses.

## Documentation

- [Usage guide](docs/Usage.md)
- [Crypto provider guide](docs/Crypto.md)
- [Contract and compatibility notes](docs/Contracts.md)

## License

MIT
