# Sandbox multisharing

This example requests `internal-passport`, `taxpayer-card`, `driver-license`,
and `vehicle-license`. Configure **one** `vehicle-license` scope in both the
branch and offer. The offer marks all four types as mandatory and the online
request selects `DSTU4145-DSTU7564` signatures. A test user who has two
vehicle registrations must select both in Diia; the online request cannot
specify a count.

Run from the repository root with the sandbox credentials and branch details
from [the examples setup](../README.md). Use a distinct branch and offer for
this test. Create each only once and keep the printed IDs. If the branch
already exists with all four scopes, set `DIIA_BRANCH_ID` to its ID and skip
`create-branch`:

```sh
go run ./examples/multisharing create-branch
export DIIA_BRANCH_ID='<printed branch_id>'
go run ./examples/multisharing inspect-branch
export DIIA_OFFER_NAME='Sandbox multisharing four types'
go run ./examples/multisharing create-offer
export DIIA_OFFER_ID='<printed offer_id>'
DIIA_SHOW_DEEPLINK=1 go run ./examples/multisharing request
export DIIA_REQUEST_ID='<printed request_id>'
go run ./examples/multisharing status
```

Use a sandbox test user with all four document types and two vehicle
registrations. The user must approve the request and select both registrations
in the test Diia app. Check the callback to confirm the actual document counts;
`initialized` only means the link was created.

If `create-offer` reports that the Diia acquirer does not support
multisharing, ask the Diia integration team to enable multisharing for the
sandbox acquirer. Keep the branch ID and rerun only `create-offer` after they
confirm enablement. Creating more branches or repeating `vehicle-license` in
the scopes will not grant that capability.

The sandbox multisharing callback observed on 2026-11-09 contained `encodeData`
and one Base64-encoded `multi-sharing.zip` field. Its encrypted ZIP directory
listed five PDF entries: one for each of the first three types and two
`vehicle-license` entries. The package uses a ZIP rather than separate
multipart document fields. ZIP entry names alone do not verify the PDF contents
or signatures. Configure the optional IIT archive
processor to open the AES ZIP and check its metadata, five PDFs, Diia seals,
and user signatures. Without that processor, `Callbacks().Process()` rejects
the ZIP callback. The local `iit/examples/decrypt-capture` tool can inspect
the captured package and, with trusted certificates and API credentials,
exercise the verified processing path.
