# Diia sandbox callback relay

This small HTTP service captures a sandbox Diia callback in Webhook.site for
inspecting its raw multipart format. It forwards only `POST
/diia/documents-receiver` to the HTTPS `WEBHOOK_URL` and returns Diia's expected
`{"success":true}` only after Webhook.site accepts the request with a 2xx status.
`GET /healthz` is available for a load balancer health check. Other paths are
not forwarded.

```sh
WEBHOOK_URL='https://webhook.site/<your-token-UUID>' go run ./tools/diia-callback-relay
```

The relay streams the original body bytes and end-to-end headers, including the
callback `Authorization` bearer token and `X-Document-Request-Trace-Id`, to
Webhook.site. It does not log their values or store the body itself. The
Webhook.site URL is a bearer-like secret: anyone who knows it can view captured
requests. Use this relay only when the Diia callback endpoint is confirmed to
receive sandbox traffic exclusively; it is not a document processor or a safe
production destination. Webhook.site's free request body limit is 10 MiB, so
larger callbacks will fail there and will not be acknowledged as successful by
this relay. The Diia user still needs to open and approve the sharing deeplink.

Build the image from the repository root:

```sh
docker build -f tools/diia-callback-relay/Dockerfile -t diia-callback-relay .
```
