# The e2e stand-in for the console's token exchange and for a cluster that
# takes a bearer token. Standard library only, so the build needs no network.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /exchange-stub .

FROM alpine:3.21
COPY --from=build /exchange-stub /exchange-stub
EXPOSE 9080 9443
ENTRYPOINT ["/exchange-stub"]
USER stubapp