# Runnable examples

These programs are small, independent examples. Run them from the repository
root with Go 1.26 or newer (use a patched Go release for integration). They
use the Diia **sandbox** by default. The SDK's
`NewClient` and `NewDefaultClient` constructors still default to production;
the examples choose sandbox explicitly.

## Configure the shell

Copy the template, fill in your sandbox credentials, and load it into your
current shell. The local file is ignored by Git; do not commit or paste tokens
into source code, tests, or command arguments.

```sh
cp examples/.env.example examples/.env
# Edit examples/.env: set DIIA_ACQUIRER_TOKEN and DIIA_AUTH_ACQUIRER_TOKEN.
set -a
. examples/.env
set +a
```

The template includes branch details that pass local validation. Change them
to represent your own test branch. Errors are reduced to a category or HTTP
status; normal output includes IDs and names, and the deeplink is opt-in.
The examples do not print credentials or raw API errors.
Values shaped like `__SET_BRANCH_ID__` are placeholders and are rejected
before any request; replace them with IDs returned by the create commands.

## Sandbox walkthrough

Run each command separately. Copy the printed IDs into your shell for later
steps. Creating a branch, offer, or sharing request changes sandbox state.
Reuse an existing test branch and offer for later runs; Diia warns that mass
creation of branches or offers can block the service. Skip the relevant create
command when reusing an ID, and delete only objects created for this walkthrough.

1. Check authentication and inspect the existing branches:

   ```sh
   go run ./examples/auth
   go run ./examples/branches list
   ```

2. Create a branch once. Set `DIIA_BRANCH_ID` to the printed `branch_id`, then
   read, update, and list it. On later runs, reuse that ID and skip `create`.
   Run `update` only for a test branch that you created and own. Diia requires
   all offers on a branch to be deleted before editing it; the example checks
   this before PUT. The command copies the current fields and sharing scopes
   known to this SDK, but PUT can still discard server fields the SDK does not
   model. `DIIA_ALLOW_BRANCH_REPLACEMENT=1` is a one-command acknowledgment of
   that risk; the example refuses to update without it.

   ```sh
   go run ./examples/branches create
   export DIIA_BRANCH_ID="<branch_id from the previous command>"
   go run ./examples/branches get
   export DIIA_BRANCH_NAME="Sandbox demo updated"
   DIIA_ALLOW_BRANCH_REPLACEMENT=1 go run ./examples/branches update
   go run ./examples/branches list
   ```

3. Create and list an offer for that branch. Set `DIIA_OFFER_ID` to the printed
   `offer_id`. Reuse this offer on later runs and skip `create`. `DIIA_RETURN_LINK`
   is optional; when set, it must be an HTTPS URL controlled by your integration.

   ```sh
   go run ./examples/offers create
   export DIIA_OFFER_ID="<offer_id from the previous command>"
   go run ./examples/offers list
   ```

4. Create an online sharing request. It prints a new `request_id` before POST
   and hides the deeplink by default. Use `DIIA_SHOW_DEEPLINK=1` on this same
   command if a test user needs to open it; every run creates another request. Set
   `DIIA_REQUEST_ID` to the printed ID, then query its status. An initial
   `initialized` status is normal before the user acts.

   ```sh
   go run ./examples/online
   export DIIA_REQUEST_ID="<request_id from the previous command>"
   go run ./examples/status online
   ```

   To display the link for a test user, replace the `online` command above
   with `DIIA_SHOW_DEEPLINK=1 go run ./examples/online`. Do not run both
   commands for the same test, because each creates a new request. If the POST
   times out or returns an ambiguous error, check the printed ID with
   `go run ./examples/status online` before retrying. A rerun generates a new
   ID and may create a second request.

5. Optionally test offline sharing when a test user can provide a fresh
   13-digit barcode from the Diia app. A made-up barcode will be rejected by
   the server. The offline command prints a new request ID before POST; replace
   `DIIA_REQUEST_ID` with the ID it prints before checking status.

   ```sh
   export DIIA_BARCODE="<fresh 13-digit barcode>"
   go run ./examples/offline
   export DIIA_REQUEST_ID="<new request_id from the previous command>"
   go run ./examples/status offline
   ```

   If the POST times out or returns an ambiguous error, check that ID with
   `go run ./examples/status offline` before retrying. A rerun generates a
   new ID; obtain a fresh barcode for a new offline request.

6. Delete only the offer and branch that you created in steps 2 and 3. Keep
   these commands even if a later sharing step fails. The SDK has no delete
   operation for a sharing request.

   ```sh
   go run ./examples/offers delete
   go run ./examples/branches delete
   go run ./examples/branches list
   ```

`go run ./examples/callback` is a separate, local demonstration of callback
parsing, processing, and responses with **synthetic** data. It does not open a
server or implement real cryptography. See [Crypto provider](../docs/Crypto.md)
for the requirements of a real callback endpoint.

For a sandbox online request with an ID card, taxpayer card, driver license,
and up to two vehicle registrations, use the [multisharing example](multisharing/README.md).
The branch and offer each list the four document types once. The user selects
both vehicle registrations in the test Diia app; the request cannot force a
per-type document count.

## Production selection

Read-only commands can target production only when `DIIA_ENV=production` is
set explicitly. Creation, update, deletion, and sharing requests additionally
require `DIIA_ALLOW_PRODUCTION_MUTATIONS=1`. Replace every sample business
field with real data before enabling production writes. `DIIA_AUTH_ACQUIRER_TOKEN`
is used only in sandbox. Invalid or missing environment values stop the
example before it sends a request. Branch update also requires
`DIIA_ALLOW_BRANCH_REPLACEMENT=1` and should be used only after verifying the
entire branch configuration against Diia's current API contract.
