# The e2e stand-in for the console's token exchange and for a cluster that
# takes a bearer token. Standard library only, so the build needs no network.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /exchange-stub .

FROM alpine:3.21
# Unprivileged: the stub listens above 1024 and needs no home directory. It
# writes only its certificate, to /certs when STUB_CERT_DIR names it; a named
# volume mounted there takes this directory's owner when first created.
RUN adduser -D -H -s /sbin/nologin stubapp \
 && install -d -o stubapp -g stubapp -m 0755 /certs
COPY --from=build /exchange-stub /exchange-stub
USER stubapp
EXPOSE 9080 9443
ENTRYPOINT ["/exchange-stub"]
