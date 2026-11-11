FROM golang:1.26.10-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY tools/diia-callback-relay ./tools/diia-callback-relay
RUN CGO_ENABLED=0 go build -trimpath -o /diia-callback-relay ./tools/diia-callback-relay

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /diia-callback-relay /diia-callback-relay
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/diia-callback-relay"]
