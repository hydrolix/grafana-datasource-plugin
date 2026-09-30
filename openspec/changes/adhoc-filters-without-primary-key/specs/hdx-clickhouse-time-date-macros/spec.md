## MODIFIED Requirements

### Requirement: PK-lookup macros resolve the column via `getPK` when omitted

The plugin SHALL define four macros that accept zero or one argument: `TimeFilter`, `TimeFilterMs`, `TimeInterval`, `TimeIntervalMs`. When `len(args) == 1 && args[0] != ""`, the macro SHALL use that column. Otherwise the macro SHALL call `getPK(ctx, query.RawSQL, pos, mdProvider, query.Headers)` to resolve the primary key for the table at `pos`. When `getPK` returns an error, the macro SHALL propagate it and SHALL NOT emit any SQL fragment, nor substitute a tautology, a literal, or an empty column name for the unresolved column. `getPK` SHALL convert an empty primary key from `MetadataProvider.GetPK` — a table that exists but declares none — into a `backend.DownstreamError` wrapping the plain sentinel `ErrPrimaryKeyEmpty` and naming the database and table, so the empty string never reaches a macro as a column. `ErrPrimaryKeyEmpty` itself SHALL carry no error source, so `errors.Is` against it is an identity check rather than a source comparison.

#### Scenario: TimeFilter with explicit column

- **GIVEN** `args = ["ts"]` and time range from `2014-11-12` to `2015-11-12`
- **WHEN** `TimeFilter(ctx, query, ["ts"], 0, mdProvider)` is called
- **THEN** the return SHALL be `"ts >= toDateTime(1415792726) AND ts <= toDateTime(1447328726)"`
- **AND** `mdProvider`'s schema cache SHALL NOT be consulted

#### Scenario: TimeFilter resolves column from MetadataProvider PK cache

- **GIVEN** SQL `SELECT $__timeFilter FROM mydb.events` with `pkCache["mydb_events"] = "primary_ts"`
- **WHEN** `TimeFilter` runs at the position of the macro
- **THEN** the return SHALL contain `"primary_ts >= toDateTime(...)" AND "primary_ts <= toDateTime(...)"`

#### Scenario: Too many arguments produces a typed error

- **GIVEN** `args = ["a", "b"]`
- **WHEN** any of `TimeFilter` / `TimeFilterMs` / `TimeInterval` / `TimeIntervalMs` is called
- **THEN** the returned error SHALL wrap `sqlutil.ErrorBadArgumentCount`
- **AND** the error SHALL be a `backend.DownstreamError`

#### Scenario: Table with no primary key produces a typed error, not SQL

- **GIVEN** SQL `SELECT $__timeFilter FROM mydb.keyless` where `mydb.keyless` exists and has no primary key
- **WHEN** any of the four PK-lookup macros runs at the position of the macro
- **THEN** the call SHALL return a non-nil error identifying the table as having no primary key
- **AND** the returned SQL fragment SHALL be empty
- **AND** no expression containing an empty column name SHALL be emitted

#### Scenario: Error message points at the explicit-column form

- **GIVEN** a zero-argument `$__timeFilter` or `$__timeInterval` over a table with no primary key
- **WHEN** the macro returns its error
- **THEN** the message SHALL name the database and table
- **AND** SHALL indicate that the time column must be passed as the macro's argument, in wording that is correct for all four PK-lookup macros
- **AND** `errors.Is(err, ErrPrimaryKeyEmpty)` SHALL be true
- **AND** `backend.IsDownstreamError(err)` SHALL be true

#### Scenario: A generic downstream lookup failure is not mistaken for a keyless table

- **GIVEN** a `MetadataProvider` whose metadata datasource returns a generic `backend.DownstreamError` for the PK query
- **WHEN** a zero-argument `$__timeFilter` runs over that table
- **THEN** the macro SHALL propagate that error
- **AND** `errors.Is(err, ErrPrimaryKeyEmpty)` SHALL be false
- **AND** the message SHALL NOT contain the explicit-column hint
