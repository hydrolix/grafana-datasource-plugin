## ADDED Requirements

### Requirement: An HTTP answer to a query is shown as the cluster wrote it

When a query fails with an HTTP answer from the cluster, the error shown SHALL start with the status as `[HTTP <status>]`. A ClickHouse exception in the answer SHALL follow as `Code: <code>. <name>: <message>`, without the name where the answer gave none; any other body SHALL follow as the cluster wrote it, neither quoted nor escaped. A JSON body that the driver cut into an exception, because the answer carried `X-ClickHouse-Exception-Code`, is a body and SHALL follow whole. The error SHALL remain a query error, so that sqlds still retries a cluster's `401`.

#### Scenario: A ClickHouse error over HTTP reads like a native one

- **GIVEN** a query error that is an HTTP `400` answer carrying ClickHouse exception code `62`, named `DB::Exception`, with a syntax error as its message
- **WHEN** it is converted for the panel
- **THEN** the message SHALL contain `[HTTP 400]`
- **AND** the message SHALL contain `Code: 62. DB::Exception:` followed by the syntax error

#### Scenario: A Turbine body reaches the panel unescaped

- **GIVEN** a query error that is an HTTP `401` answer whose body is the JSON `{"error": "<TurbineApiAuthenticatorError …>"}`, with no ClickHouse exception
- **WHEN** it is converted for the panel
- **THEN** the message SHALL contain the body exactly as the cluster wrote it

#### Scenario: A Turbine body announced by the exception header reaches the panel whole

- **GIVEN** a cluster that answers a query with HTTP `400`, `X-ClickHouse-Exception-Code: 47` and the JSON body `{"error": "Code: 47. DB::Exception: Missing columns: … (UNKNOWN_IDENTIFIER)", "query": "…"}`
- **WHEN** the query's error is converted for the panel
- **THEN** the message SHALL be `[HTTP 400] response body: ` followed by the body exactly as the cluster wrote it
- **AND** it SHALL equal the message for the same answer without the header

#### Scenario: A code that only the header gives

- **GIVEN** a cluster that answers a query with HTTP `400`, `X-ClickHouse-Exception-Code: 1001` and the JSON body `{"error": "internal authentication failure", "query": "…"}`
- **WHEN** the query's error is converted for the panel
- **THEN** the message SHALL be `[HTTP 400] Code: 1001. ` followed by the body

#### Scenario: A cluster 401 carrying an exception is still retried

- **GIVEN** a query error that is an HTTP `401` answer carrying ClickHouse exception code `516`
- **WHEN** it is converted for the panel
- **THEN** the message SHALL contain `[HTTP 401]`
- **AND** `errors.Is(err, sqlds.ErrorQuery)` SHALL report true

### Requirement: A ClickHouse error reads the same over either protocol

When a panel shows a ClickHouse exception, of the class `DB::Exception` or another `DB::…Exception`, it SHALL show the exception's message alone, without the code, the exception's class, the error name in parentheses with the period before it, or the server version. A native exception, which carries no error name in parentheses and no version, SHALL read the same as the same exception over HTTP. A word in parentheses that no period precedes, such as the token Turbine names in a syntax error, SHALL stay part of the message. Where an exception without an error name wraps a Turbine database error in angle brackets, the message SHALL be the database error's.

#### Scenario: A native exception is shown by its message

- **GIVEN** a query error `Code: 60. DB::Exception: Unknown table expression identifier 'nope' in scope SELECT * FROM nope`, as a native query returns it
- **WHEN** the panel shows it
- **THEN** the message SHALL be `Unknown table expression identifier 'nope' in scope SELECT * FROM nope`

#### Scenario: An HTTP exception is shown by its message

- **GIVEN** a query error `[HTTP 400] Code: 62. DB::Exception: Syntax error: failed at position 1 ('SELEC'): SELEC 1. Expected one of: Query. (SYNTAX_ERROR)`
- **WHEN** the panel shows it
- **THEN** the message SHALL be `Syntax error: failed at position 1 ('SELEC'): SELEC 1. Expected one of: Query`

#### Scenario: A Turbine syntax error keeps its token

- **GIVEN** a query error whose body is the JSON `{"error": "Code: 62. DB::Exception: Syntax error: failed at position 1 (SELEC): SELEC 1. Expected one of: Query. (SYNTAX_ERROR)", "query": "SELEC 1"}`
- **WHEN** the panel shows it
- **THEN** the message SHALL be `Syntax error: failed at position 1 (SELEC): SELEC 1. Expected one of: Query`

#### Scenario: An exception of another class

- **GIVEN** a query error `Code: 210. DB::NetException: Connection reset by peer, while reading from socket`
- **WHEN** the panel shows it
- **THEN** the message SHALL be `Connection reset by peer, while reading from socket`

#### Scenario: A Turbine database error inside a native exception

- **GIVEN** a query error `Code: 1001. DB::Exception: h::db::ChTurbineDatabaseError: <ChTurbineDatabaseError table unknown not found>`
- **WHEN** the panel shows it
- **THEN** the message SHALL be `table unknown not found`

#### Scenario: A query repeated in a Turbine body is not left escaped

- **GIVEN** a query error `[HTTP 400] response body: ` followed by a JSON body whose `error` repeats a query holding `\"a\"`
- **WHEN** the panel shows it
- **THEN** the message SHALL hold `"a"` with no backslash
