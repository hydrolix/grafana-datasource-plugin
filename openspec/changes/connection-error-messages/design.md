## Context

Three paths carry a connection error to a person, and only one of them passes through code the plugin can reach after the fact.

1. **Instance build.** Saving a data source makes Grafana build a new instance; sqlds calls `Connect`, which pings. When the ping fails the instance is not built, and the SDK's instance manager answers the health check, and every query, with that error's text. `HdxSqlDatasource.CheckHealth` is never called.
2. **Health check on a built instance.** sqlds pings the connection it holds and answers with the ping's error as a string.
3. **Query on a built instance.** The error goes through `MutateQueryError`. The forwarding credential modes skip the ping in `Connect`, so their first dial is here, and their health check never dials at all.

Measured on the dev stack before the change, each path returned text such as `dial tcp 10.255.255.1:9000: i/o timeout`, `lookup nohost.invalid on 127.0.0.11:53: no such host`, a `Post "http://u:***@host:port?…"` URL, `[HTTP 404] response body: "…"` from a non-cluster service, and `redirecting to "http://keycloak:8080/admin/"`.

Grafana's review named path 1's symptom and suggested fixing it in `CheckHealth`, which is path 2.

The clickhouse-go fork's v2.51.0 branch changes what paths 1 and 3 receive over HTTP. A non-200 answer is a typed `clickhouse.HTTPError` holding the status. Inside it is a `clickhouse.Exception` when the body starts with `Code: N.` or the answer carries `X-ClickHouse-Exception-Code`, and otherwise an error whose text is `response body: ` and the body quoted with `%q`. Path 2 still gets the same error as a string.

## Goals / Non-Goals

**Goals:**
- No address, port, resolver, URL, user name, response body or redirect target in a connection failure shown to a person.
- Each failure still says which kind it was.
- The original error is in the server log.

**Non-Goals:**
- Hiding what the cluster answers to a query. A SQL error arrives over HTTP as status-and-body text, and its author needs it.
- A cancelled query. sqlds returns its error without calling `MutateQueryError`.
- New settings to turn the detail back on.

## Decisions

### D1. Classify in `Connect`, not only in `CheckHealth`

`Connect` returns the fixed message at the ping failure. That is the only place that covers path 1, and it covers a query on a data source that never connected at the same time.

- Alternative considered: replace the message in `CheckHealth` only, as the review suggested. Rejected because that method is not reached when the instance fails to build, which is the case a changed host always produces.

### D2. `CheckHealth` replaces an error result's message as well

For path 2 the wrapper passes sqlds's message through the same classifier. Every error result is replaced, not only those that look like a connection error: an unrecognised message becomes the generic sentence, so nothing passes by not matching. A refused redirect is the one message that is recognised and kept: it already names the status alone (D7).

### D3. `MutateQueryError` replaces transport failures only

A query error is replaced when it is a resolve, dial, TLS or timeout failure, or carries a network or URL error type. An answer from the cluster is never replaced by a fixed sentence; D8 covers how an HTTP one is printed. Without this the forwarding modes return the dial error from a panel while their health check reports OK.

- Alternative considered: leave panels alone, as the review did not ask. Rejected because the person who can probe through a panel is the one who can probe through "Save & test", and in a forwarding mode the output is identical.
- Alternative considered: also strip HTTP response bodies from query errors. Rejected because that is how a real cluster's SQL error arrives; see Risks.

### D4. One error type that prints the message and unwraps to the cause

sqlds decides whether to retry with `errors.Is(err, ErrorQuery)` and `errors.Is(err, context.DeadlineExceeded)`. The fixed message is an error whose `Error()` is the sentence and whose `Unwrap()` is the original, so those checks hold and nothing prints the cause. The same type carries the text of D8, for the same reason.

### D5. Recognise by type first, by text where no type exists

Kinds are matched with `errors.As` / `errors.Is` (`net.DNSError`, `net.OpError`, `syscall.ECONNREFUSED`, the `crypto/tls` error types, `http.ErrSchemeMismatch`, `clickhouse.HTTPError`, `clickhouse.Exception`). Text matching is used in seven places where there is no type to ask:

- sqlds hands the health check a string, not an error;
- the fork's HTTP error reaches the health check as that string, `[HTTP 403] …`;
- inside a typed HTTP error, a body that is not an exception is known only by the fork's `response body: ` in front of it;
- inside a typed HTTP error, an exception that the header announced and whose body could not be read is known only by its message, `failed to read response body: …`;
- several TLS handshake failures are plain `errors.New("tls: …")`;
- a ClickHouse exception that reached the health check as a string is read from `code: N, message:`;
- a refused redirect that reached the health check as a string is read from the opening of `RedirectError`'s own message.

This departs from the project rule against string matching for control flow. The departure is confined to choosing a message: a text that matches nothing gets the generic sentence, so a mismatch costs precision, never a leak. The unread exception is the one place where that does not hold by construction: were the fork to word that message differently, the read error would be printed as an exception's message. A test takes that error from the fork itself (see Risks).

An HTTP status is read before everything else, because what comes with it is a body someone else wrote: it could contain any of the phrases, or look like an exception or a redirect, and the fork parses any body that starts with `Code: N.` into a typed exception. At connect the order is: typed redirect, typed HTTP answer, typed exception, HTTP status text, exception text, redirect text, transport kinds. On a query, an HTTP answer whose body arrived is never classified as transport; one whose body could not be read, with or without an announced exception, still is.

A `net.DNSError` whose lookup timed out is a timeout, not an unresolved name: the name may be a good one.

### D6. At connect, an answer is reduced to its status or code

An HTTP answer becomes "Authentication failed" for 401 and 403 and "The server answered HTTP *n*" otherwise. A ClickHouse exception becomes "Authentication failed" for codes 192, 193, 194 and 516 and "The server refused the connection (ClickHouse error code *n*)" otherwise. Neither repeats the body or the exception text. A deadline at connect is always "Connection timed out"; on a query, a timeout that is not a dial is "The request timed out".

### D7. A redirect is reported by status alone

`RedirectError` keeps its redacted `Location` for the log and drops it from the message. This reverses the 0.13.x choice to show scheme, host and path: the target is written by whatever answers at the configured address, which on a shared Grafana can be a service the person has no other view of.

### D8. A query's HTTP answer is printed as its status and what the cluster wrote

`MutateQueryError` reads a typed HTTP answer before the exception it may hold, and prints `[HTTP n]` followed by `Code: N. Name: message` for an exception, or by `response body: ` and the body with the fork's quoting undone. The status is put in front through the fork's own type, so the form the health check and the exchange-mode retry marker read has one author. The original error stays underneath (D4).

Turbine sends `X-ClickHouse-Exception-Code` with some of its JSON answers. The fork then cuts the body at `Code: N.` and parses the remainder as an exception, whose text ends in the JSON's tail and is still escaped as a JSON string. Where putting `{"error": "` back in front makes that text valid JSON, it is printed as a body, so the same answer reads the same with and without the header.

- Alternative considered: let the existing exception branch take it. Rejected because that prints the exception alone and drops both the status and the chain, which sqlds's retry of an exchange-mode `401` reads.
- Alternative considered: pass the fork's text through. Rejected because `%q` escapes the quotes of a JSON body, and the frontend then cannot parse it.
- Alternative considered: undo the cut in the frontend. Rejected because the cut is the fork's, and what the fork does belongs next to its pin.

### D9. The beautifier ends a message at the error name, or at the end of the text

ClickHouse puts `. (NAME)` after an HTTP exception's message and nothing after a native one. The beautifier ends the message at a parenthesised word that a period precedes, for any `DB::…Exception` class; a text without one is tried against the older patterns and then taken to its end. So a native exception reads like the same one over HTTP.

- Alternative considered: end at the first parenthesised word, as before. Rejected because Turbine writes the failing token of a syntax error that way, unquoted, and the message was cut there.

## Risks / Trade-offs

- [An operator cannot tell an expired certificate from an untrusted one, or see a cluster's own refusal text, in the UI] → The original is logged at error level once per failure (`connecting to the cluster failed`, `health check failed`, `query did not reach the cluster`). On Grafana Cloud the operator cannot read that log; accepted.
- [A response body still reaches a panel in a forwarding mode when the data source points at a non-cluster HTTP service] → Not mitigated. The typed HTTP error says that something answered, not whether it was a cluster.
- [A fork release words `response body: ` or `failed to read response body: ` differently, or shapes an answer differently] → `TestMutateQueryErrorOnTheForksOwnErrors` takes each error from the fork through a stub cluster and compares the whole message; the unread exception fails there before it can print a read error.
- [Something that is not a Turbine answer ends like one and is printed as a body] → The text must be valid JSON as a whole once the opening is back, with a non-empty `error`; `TestMutateQueryErrorShowsAnHTTPAnswerAsTheClusterWroteIt` holds an exception that ends in `"}` and is left alone.
- [A sqlds upgrade changes the health check's text, or a later edit drops the remap] → `TestHealthCheckOfALostClusterNamesNoAddress` builds an instance through a TCP relay to the test container, closes the relay, and fails on any address in the message.
- [A new failure kind is misfiled] → It falls to the generic sentence. `TestConnectFailureRepeatsNothingOfWhatAnswered` asserts no network detail in any message.
- [`CheckHealth` overwrites a message from a future pre- or post-check hook] → None is set today. A hook added later must return before the remap or be exempted.
