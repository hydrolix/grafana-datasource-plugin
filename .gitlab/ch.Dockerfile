FROM clickhouse/clickhouse-server:latest

RUN sed -i 's|<custom_settings_prefixes>SQL_</custom_settings_prefixes>|<custom_settings_prefixes>hdx_</custom_settings_prefixes>|' /etc/clickhouse-server/config.xml