# The e2e stand-in for the console's token exchange and for a cluster that
# takes a bearer token. Standard library only, so the build needs no network.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /exchange-stub .

FROM alpine:3.21
# Unprivileged: the stub listens above 1024 and keeps its certificate in
# memory, so it needs no home directory and writes nothing.
RUN adduser -D -H -s /sbin/nologin stubapp
COPY --from=build /exchange-stub /exchange-stub
USER stubapp
EXPOSE 9080 9443
ENTRYPOINT ["/exchange-stub"]
