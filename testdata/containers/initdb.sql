CREATE DATABASE IF NOT EXISTS e2e;
DROP TABLE IF EXISTS e2e.macros;
CREATE TABLE e2e.macros
(
    datetime DateTime64(3, 'UTC'),
    date Date,
    v1 INT
) ENGINE = MergeTree() ORDER BY datetime;

INSERT INTO e2e.macros (datetime, date, v1) VALUES
    ('2025-04-08 23:59:59', '2025-04-08', -1),
    ('2025-04-09 00:00:00', '2025-04-09', 0),
    ('2025-04-09 00:10:00', '2025-04-09', 1000),
    ('2025-04-09 00:20:00', '2025-04-09', 2000),
    ('2025-04-09 00:30:00', '2025-04-09', 3000),
    ('2025-04-09 00:40:00', '2025-04-09', 4000),
    ('2025-04-09 00:50:00', '2025-04-09', 5000),
    ('2025-04-09 01:00:00', '2025-04-09', 10000),
    ('2025-04-09 01:10:10', '2025-04-09', 11010),
    ('2025-04-09 01:10:30', '2025-04-09', 11030),
    ('2025-04-09 01:11:00', '2025-04-09', 11100),
    ('2025-04-10 00:10:00', '2025-04-10', 1000),
    ('2025-04-10 00:20:00', '2025-04-10', 2000),
    ('2025-04-10 00:30:00', '2025-04-10', 3000),
    ('2025-04-11 00:00:01', '2025-04-11', -1);

DROP TABLE IF EXISTS e2e.datatypes;
CREATE TABLE e2e.datatypes
(
    datetime  DateTime64(3, 'UTC'),
    uuid_col  UUID,
    uuid_null Nullable(UUID),
    v4_col    IPv4,
    v4_null   Nullable(IPv4),
    v6_col    IPv6,
    v6_null   Nullable(IPv6),
    -- An IPv6 column holding IPv4-mapped addresses. This mirrors how Hydrolix
    -- stores its own `ip` transform type: one IPv6 column carrying both
    -- families, IPv4 padded to ::ffff:a.b.c.d. Rendering is type-driven:
    -- the plugin keeps the ::ffff: prefix, matching
    -- ClickHouse's own toString() form, so panel text round-trips into every
    -- ad-hoc filter operator.
    v6_mapped IPv6,
    -- An IPv6 column holding the deprecated IPv4-compatible form (::a.b.c.d,
    -- no ffff). ClickHouse renders it with a dotted-quad tail where Go's netip
    -- renders hex ("::a00:1"), so this column exists to pin that branch of the
    -- renderer end to end. It matters because '=~' compares toString(column):
    -- a hex rendering in the panel would be text the server never produces, so
    -- the value a user copies out of a cell would match no rows.
    v6_compat IPv6
) ENGINE = MergeTree() ORDER BY datetime;

INSERT INTO e2e.datatypes (datetime, uuid_col, uuid_null, v4_col, v4_null, v6_col, v6_null, v6_mapped, v6_compat) VALUES
    ('2025-04-09 00:00:00', '61f0c404-5cb3-11e7-907b-a6006ad3dba0', '61f0c404-5cb3-11e7-907b-a6006ad3dba0', '1.2.3.4', '1.2.3.4', '2001:db8::1', '2001:db8::1', '::ffff:1.2.3.4', '::10.0.0.1');
INSERT INTO e2e.datatypes (datetime, uuid_col, uuid_null, v4_col, v4_null, v6_col, v6_null, v6_mapped, v6_compat) VALUES
    ('2025-04-09 00:10:00', '9d3b1f3a-0c2e-4c9a-9a8b-1c2d3e4f5a6b', NULL, '5.6.7.8', NULL, '2001:db8::2', NULL, '::ffff:5.6.7.8', '::20.0.0.2');

-- Ad-hoc filter value preload fixture (tests/adHocFilterValues.spec.ts,
-- tests/adHocGuardrails.spec.ts). Skewed-frequency `status` (dominant
-- "common" + rare tail + empty strings), a Nullable and a non-Nullable
-- mirror column, Array/Map columns, and one "old_only" row pinned 1h
-- before the trailing-24h preload window (2025-04-19T00:00Z..04-20T00:00Z).
DROP TABLE IF EXISTS e2e.adhoc_topk;
CREATE TABLE e2e.adhoc_topk
(
    ts DateTime64(3, 'UTC'),
    status String,
    status_null Nullable(String),
    status_nonnull String,
    tags Array(String),
    attrs Map(String, Nullable(String))
) ENGINE = MergeTree() ORDER BY ts;

INSERT INTO e2e.adhoc_topk
SELECT toDateTime64('2025-04-19 00:00:00', 3, 'UTC') + INTERVAL number MINUTE,
       'common',
       if(number % 5 = 0, NULL, 'common'),
       'common',
       ['common', 'tagB'],
       map('env', 'prod')
FROM numbers(100);

INSERT INTO e2e.adhoc_topk (ts, status, status_null, status_nonnull, tags, attrs) VALUES
    ('2025-04-18 23:00:00', 'old_only', 'old_only', 'old_only', [], map()),
    ('2025-04-19 10:00:00', 'rare1', 'rare1', 'rare1', [], map()),
    ('2025-04-19 10:05:00', 'rare2', 'rare2', 'rare2', [], map()),
    ('2025-04-19 10:10:00', 'rare3', 'rare3', 'rare3', [], map()),
    ('2025-04-19 11:00:00', '', '', '', [], map()),
    ('2025-04-19 11:05:00', '', '', '', [], map());

-- Slow ad-hoc value source (tests/adHocGuardrails.spec.ts slow-source
-- test): 100 rows in ONE insert (=> one part) so a sleepEachRow predicate
-- reads them sequentially; index_granularity = 10 lets a max_block_size=10
-- query setting split the scan into ~10-row blocks, keeping each
-- sleepEachRow call under ClickHouse's 3s-per-call TOO_SLOW cap while the
-- total scan still takes ~10s.
DROP TABLE IF EXISTS e2e.adhoc_slow_src;
CREATE TABLE e2e.adhoc_slow_src
(
    ts DateTime64(3, 'UTC'),
    status String
) ENGINE = MergeTree() ORDER BY ts SETTINGS index_granularity = 10;

INSERT INTO e2e.adhoc_slow_src
SELECT toDateTime64('2025-04-19 00:00:00', 3, 'UTC') + INTERVAL number MINUTE,
       'slowval'
FROM numbers(100);

-- Ad-hoc filter value preload fixture (tests/adHocFilterValues.spec.ts,
-- tests/adHocGuardrails.spec.ts). Skewed-frequency `status` (dominant
-- "common" + rare tail + empty strings), a Nullable and a non-Nullable
-- mirror column, Array/Map columns, and one "old_only" row pinned 1h
-- before the trailing-24h preload window (2025-04-19T00:00Z..04-20T00:00Z).
DROP TABLE IF EXISTS e2e.adhoc_topk;
CREATE TABLE e2e.adhoc_topk
(
    ts DateTime64(3, 'UTC'),
    status String,
    status_null Nullable(String),
    status_nonnull String,
    tags Array(String),
    attrs Map(String, Nullable(String))
) ENGINE = MergeTree() ORDER BY ts;

INSERT INTO e2e.adhoc_topk
SELECT toDateTime64('2025-04-19 00:00:00', 3, 'UTC') + INTERVAL number MINUTE,
       'common',
       if(number % 5 = 0, NULL, 'common'),
       'common',
       ['common', 'tagB'],
       map('env', 'prod')
FROM numbers(100);

INSERT INTO e2e.adhoc_topk (ts, status, status_null, status_nonnull, tags, attrs) VALUES
    ('2025-04-18 23:00:00', 'old_only', 'old_only', 'old_only', [], map()),
    ('2025-04-19 10:00:00', 'rare1', 'rare1', 'rare1', [], map()),
    ('2025-04-19 10:05:00', 'rare2', 'rare2', 'rare2', [], map()),
    ('2025-04-19 10:10:00', 'rare3', 'rare3', 'rare3', [], map()),
    ('2025-04-19 11:00:00', '', '', '', [], map()),
    ('2025-04-19 11:05:00', '', '', '', [], map());

-- Slow ad-hoc value source (tests/adHocGuardrails.spec.ts slow-source
-- test): 100 rows in ONE insert (=> one part) so a sleepEachRow predicate
-- reads them sequentially; index_granularity = 10 lets a max_block_size=10
-- query setting split the scan into ~10-row blocks, keeping each
-- sleepEachRow call under ClickHouse's 3s-per-call TOO_SLOW cap while the
-- total scan still takes ~10s.
DROP TABLE IF EXISTS e2e.adhoc_slow_src;
CREATE TABLE e2e.adhoc_slow_src
(
    ts DateTime64(3, 'UTC'),
    status String
) ENGINE = MergeTree() ORDER BY ts SETTINGS index_granularity = 10;

INSERT INTO e2e.adhoc_slow_src
SELECT toDateTime64('2025-04-19 00:00:00', 3, 'UTC') + INTERVAL number MINUTE,
       'slowval'
FROM numbers(100);
