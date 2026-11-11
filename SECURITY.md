# Security policy

Please report security issues privately to the repository owner instead of
opening a public issue. Do not include production tokens, callback payloads,
metadata, PDFs, certificates, private keys, or passwords in a report.

Use a currently supported patched Go toolchain in production. The `go 1.26`
directive defines language compatibility; use a patched Go 1.26 release.

Callbacks must use HTTPS, strict request-ID replay protection in partner
storage, an audited `CryptoProvider`, and generic failure responses. Configure
a bearer token when Diia sends one. For a callback without `Authorization`,
explicitly authorize an active pending request ID and process only a verified
multi-sharing ZIP; a present invalid token must always fail. Treat
`APIError.RawBody` and `DocumentPackage.RawMetadata` as sensitive data.
