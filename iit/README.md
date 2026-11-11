# Optional IIT provider for go-diia

This separate module provides the IIT crypto provider for encrypted Diia
callbacks. Applications import it alongside the root `go-diia` package. The
root SDK has no native IIT dependency.

```go
import (
    diia "github.com/stremovskyy/go-diia"
    diiaiit "github.com/stremovskyy/go-diia/iit"
)

config, err := diiaiit.LoadConfig().
    WithPrivateKeyFile(keyPath, password).
    WithCASettingsFile(caSettingsPath).
    WithCACertificatesFile(caCertificatesPath).
    WithPartnerCertificateFiles(partnerCertificatePaths...).
    WithDiiaEncryptionCertificateFiles(diiaEncryptionCertificatePaths...).
    WithDiiaSigningCertificateFiles(diiaSigningCertificatePaths...).
    Do(ctx)
if err != nil {
    return err
}

provider, err := diiaiit.NewProvider().WithConfig(config).Do(ctx)
config.Destroy()
if err != nil {
    return err
}
defer func() { _ = provider.Close().Do(context.Background()) }()

client := diia.NewClient(acquirerToken,
    diia.WithCallbackToken(callbackToken),
    diia.WithCryptoProvider(provider),
    diia.WithMultisharingArchiveProcessor(diiaiit.NewMultisharingArchiveProcessor(provider)),
)
```

For multi-sharing, the SDK first verifies and decrypts `encodeData`, then
fetches the ZIP password with the same authenticated Diia session. This
processor opens the WinZip AES-256 archive in memory, authenticates each entry
before using its plaintext, verifies attached Diia CMS seals and detached user
signatures against the exact metadata and PDF bytes, and matches every PDF to
`metadata.data.<type>[i].fileName`. Repeated document types remain separate
`DocumentFile` values. Configure current trusted Diia signing certificates and
the IIT CA and online status settings; a missing or untrusted signer fails the
callback. The archive is capped at 16 MiB, 128 entries, 32 MiB per unpacked
entry, and 128 MiB unpacked total. ZIP entry names are never used as filesystem
paths.

To check an encrypted sample ZIP's format locally without making a signer-trust
assertion, set `DIIA_MULTISHARING_SAMPLE_ZIP` to its local path and
`DIIA_MULTISHARING_SAMPLE_PASSWORD` to its password, then run
`go test -run TestPasswordedSampleZIPFormat -v .` from
this module. This optional test uses a permissive verifier only for file
layout and linkage. Production processing requires an audited verifier
configured with trusted certificates; the supplied `Provider` supports this
path.

`Provider` uses the official `iit/euscp` runtime. Obtain and install the locked
archive using `go-diia-iit/cmd/setup-iit`, then make the installed native module
available to the final application through a local Go workspace or a `replace`
in that application's `go.mod`. The official module is not available from the
public Go proxy. On Linux, set `LD_LIBRARY_PATH` to its `lib` directory.

For local development before publication, a workspace must include `go-diia`,
`go-diia/iit`, `go-diia-iit`, and the installed `iit/euscp`. The facade's
`go.mod` expects `v0.1.0` of `go-diia` and `go-diia-iit`; publish those
versions before tagging this nested module as `iit/v0.1.0`.
The root [CI workflow](../.github/workflows/ci.yml) runs on pushes and pull
requests without IIT. The [optional IIT integration
workflow](../.github/workflows/iit.yml) runs only through GitHub Actions
`workflow_dispatch`. Before running it, make the pinned `go-diia-iit` commit
available and set the repository secrets `IIT_RUNTIME_ARCHIVE_URL` and
`IIT_FIXTURES_ARCHIVE_URL` to HTTPS download URLs for the exact official
archives recorded in the companion module's `upstream.lock.json`. Obtain the
archives and access URLs from IIT; the public URLs in that lock currently
return 404. The workflow downloads into the temporary runner directory and
passes each file to the setup command for size, SHA-256, and content checks.
It does not publish or cache the archives.

The companion module also tracks a [Darwin arm64 installed-file
snapshot](https://github.com/stremovskyy/go-diia-iit/blob/main/docs/Installation.md#checking-the-installed-darwin-arm64-runtime).
Its verifier checks the local runtime files without the original ZIP. This
does not authenticate the ZIP or cover the sharing fixtures.

For a newer IIT release, follow the companion module's [archive update
procedure](https://github.com/stremovskyy/go-diia-iit/blob/main/docs/Installation.md#updating-the-locked-archives)
first. After its archive checks and native tests pass, update the full companion
commit SHA in [the optional workflow](../.github/workflows/iit.yml), review
`iit/go.mod` for changed module requirements, and rerun this module's tests
without `go.work` using a local replacement for the official `iit/euscp`
module. Release new compatible versions in dependency order; do not move
published tags.

With `go-diia-iit` checked out beside `go-diia`, create a local workspace from
the `go-diia` directory:

```bash
go work init . ./iit ../go-diia-iit ../go-diia-iit/iit/euscp
go work edit -go=1.26 \
  -replace=github.com/stremovskyy/go-diia@v0.1.0=. \
  -replace=github.com/stremovskyy/go-diia-iit@v0.1.0=../go-diia-iit \
  -replace=iit/euscp@v1.3.1=../go-diia-iit/iit/euscp
```

The local replacements also let IDEs that call `go list -m all` resolve the
unpublished versions and the official IIT module. `go.work` is ignored because
its sibling paths depend on the local checkout layout.

See [the callback integration template](examples/callback) for HTTP wiring. It
deliberately rejects every callback until the application implements a durable,
atomic request claim and storage for the verified documents.
