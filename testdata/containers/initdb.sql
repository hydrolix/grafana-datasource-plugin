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
    v6_mapped IPv6
) ENGINE = MergeTree() ORDER BY datetime;

INSERT INTO e2e.datatypes (datetime, uuid_col, uuid_null, v4_col, v4_null, v6_col, v6_null, v6_mapped) VALUES
    ('2025-04-09 00:00:00', '61f0c404-5cb3-11e7-907b-a6006ad3dba0', '61f0c404-5cb3-11e7-907b-a6006ad3dba0', '1.2.3.4', '1.2.3.4', '2001:db8::1', '2001:db8::1', '::ffff:1.2.3.4');
INSERT INTO e2e.datatypes (datetime, uuid_col, uuid_null, v4_col, v4_null, v6_col, v6_null, v6_mapped) VALUES
    ('2025-04-09 00:10:00', '9d3b1f3a-0c2e-4c9a-9a8b-1c2d3e4f5a6b', NULL, '5.6.7.8', NULL, '2001:db8::2', NULL, '::ffff:5.6.7.8');
