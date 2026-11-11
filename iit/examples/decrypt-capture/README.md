# Inspect one captured sandbox callback

This local diagnostic retrieves the parsed fields of a callback already saved
by webhook.site. `inspect` checks the trace ID and decodes the Base64 IIT
envelopes without a private key. `decrypt` processes separate document parts
or a multi-sharing ZIP with the partner's matching sandbox private key, Diia
certificates, and, for ZIP, sandbox API credentials.
The `example-...` trace ID identifies the sharing request; it is not a key.

A live multisharing callback can instead contain one `multi-sharing.zip` file
plus `encodeData`. In that case `inspect` reads only the ZIP directory and
prints counts of visible PDF entry names by document type, the entry count,
and how many entries carry the ZIP encryption flag. It never prints archive
entry names or file contents. These counts are **unverified**: the archive
entries and signatures have not been decrypted or checked. ZIP paths and
declared sizes are bounded before reporting them. `decrypt` verifies
`encodeData`, fetches the password by `offerRequestId`, authenticates the AES
ZIP, and checks metadata and PDF signatures and their one-to-one links.

From the repository's `iit` directory, set the three identifiers in your
local shell (do not put the webhook.site URL token in source control):

```sh
export WEBHOOK_SITE_TOKEN='<UUID from the webhook.site URL>'
export WEBHOOK_SITE_REQUEST='<UUID of the captured webhook.site request>'
export DIIA_REQUEST_ID='<X-Document-Request-Trace-Id from that request>'
go run ./examples/decrypt-capture inspect
```

For `decrypt`, also set `PARTNER_KEY_PATH`, `PARTNER_KEY_PASSWORD` (or
`PARTNER_KEY_PASSWORD_FILE` pointing to a mode `0600` local file),
`IIT_CA_SETTINGS_PATH`, `IIT_CA_CERTIFICATES_PATH`,
`PARTNER_CERTIFICATE_PATHS` (if supplied),
`DIIA_ENCRYPTION_CERTIFICATE_PATHS`, and
`DIIA_SIGNING_CERTIFICATE_PATHS` as in
[the live IIT callback example](../callback/main.go). The private key must
match the partner certificate registered for this sandbox callback. The
official IIT fixture key in `go-diia-iit/testdata` is not a substitute.
An IIT `.ZS2` personal key file can be passed directly as `PARTNER_KEY_PATH`;
it does not need conversion based on its extension. Obtain sandbox CA settings,
the certificate bundle, and current Diia encryption and signing certificates
from your authorized integration contact. The certificates in the official IIT
test fixtures are not Diia certificates. Reserve certificates may not match
the saved sandbox callback; use the active sandbox certificates for this
integration.
Keep the password in a local secret store or protected environment; do not
paste it into chat, source, or shell history. Delete a temporary password file
after the diagnostic run.

For a multi-sharing ZIP, also set `DIIA_ACQUIRER_TOKEN` and
`DIIA_AUTH_ACQUIRER_TOKEN`. The command uses the sandbox API to retrieve the
ZIP password. It never prints that password or the `offerRequestId`. Use Diia
encryption and signing certificates obtained independently from the Diia
integration team. Certificates extracted from a callback are useful only for
format diagnostics and cannot establish that the signer belongs to Diia.
Set `DIIA_MANDATORY_SHARING` to a comma-separated list of the pending offer's
mandatory document types to reject a verified archive missing any of them.

```sh
go run ./examples/decrypt-capture decrypt
```

Successful decryption prints only request ID, document types, and sizes. To
save verified metadata and PDFs, set `DIIA_OUTPUT_DIR` to a **new** directory
outside the repository before `decrypt`. It is created with mode `0700` and
files with mode `0600`; the command will not overwrite an existing directory.
The output contains personal data.

The webhook.site `/raw` download for this multipart capture is empty. This
example reconstructs the individual decoded fields from webhook.site's parsed
form and file download. It cannot recover the original MIME body or boundaries.
For a ZIP callback, `decrypt` re-encodes those fields into a local multipart
request and runs `Callbacks().Parse()` with the documented missing-Bearer mode,
using the expected active `DIIA_REQUEST_ID`. This checks the parser path but
cannot prove the original request's unavailable bytes or headers. A real
receiver must authenticate callbacks, match a pending request ID, verify with
IIT, save the documents durably, and only then acknowledge success.
