# Diia sandbox callback relay

This small HTTP service forwards sandbox Diia callbacks to an explicitly
allowed HTTPS destination. It forwards only `POST /diia/documents-receiver`
to `WEBHOOK_URL` and returns Diia's expected `{"success":true}` only after the
destination accepts the request with a 2xx status.
`GET /healthz` is available for a load balancer health check. Other paths are
not forwarded.

```sh
WEBHOOK_URL='https://webhook.site/<your-token-UUID>' go run ./tools/diia-callback-relay
```

For the Wormhole DEV callback, set:

```sh
WEBHOOK_URL='https://wormhole.dev-miranda.megakit.pro/diia/callback' go run ./tools/diia-callback-relay
```

Only these two destination forms are accepted. User information, alternate
ports, query strings, fragments, encoded paths, and redirects are rejected.
The Webhook.site target keeps a 25-second upstream timeout; the Wormhole DEV
target allows up to 180 seconds for callback verification.

The relay streams the original body bytes and end-to-end headers, including the
callback `Authorization` bearer token and `X-Document-Request-Trace-Id`, to
the chosen destination. It does not log their values or store the body itself.
The Webhook.site URL is a bearer-like secret: anyone who knows it can view
captured requests. Use this relay only when the Diia callback endpoint is
confirmed to receive sandbox traffic exclusively; it is not a document
processor or a safe production destination. Webhook.site's free request body
limit is 10 MiB, so larger callbacks will fail there and will not be
acknowledged as successful by this relay. The Diia user still needs to open
and approve the sharing deeplink.

Build the image from the repository root:

```sh
docker build -f tools/diia-callback-relay/Dockerfile -t diia-callback-relay .
```
