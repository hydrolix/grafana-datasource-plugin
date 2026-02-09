import { ErrorTemplate } from "../types";

export const SOLUTION_TEMPLATES: ErrorTemplate[] = [
  {
    code: 1001,
    name: "TIMERANGE_EXCEEDED",
    regexp:
      "<HdxStorageError Maximum time range exceeded for query: (?<time_range_seconds>.+?) seconds \\(maximum is (?<max_time_range_seconds>.+?)\\)>",
    template:
      "The error indicates that the query's time range ({time_range_seconds} seconds) exceeds the maximum allowed window of {max_time_range_seconds} seconds.\n\nHere's what you can do to resolve this:\n- Reduce the query's time filter to a range of {max_time_range_seconds} seconds or less. Note that for optimal query performance, select the smallest time range that makes sense.\n- If you need data covering a period longer than {max_time_range_seconds} seconds, try running multiple queries (for example, one per {max_time_range_seconds} seconds) and then merge or aggregate the results.\n- If longer lookback periods are required, please contact your Hydrolix representative to check whether the maximum query time range can be increased\n\nIn short, this is not a syntax or connectivity issue; it's a query constraint set at the storage or query engine level.",
  },
  {
    code: 210,
    name: "NETWORK_ERROR",
    regexp:
      "I/O error: Broken pipe, while writing to socket \\((?<server_address>.+?) -> (?<client_address>.+?)\\)",
    template:
      'The error "Broken pipe" indicates that the connection between the client ({client_address}) and the DB server ({server_address}) was interrupted while data was being written to the socket.\n\nPossible causes and steps to check:\n- The server (DB) or client process may have closed the connection unexpectedly.\n- There may be intermittent network instability or a temporary disconnection.\n- If your query returns a large dataset, the connection may time out or run out of memory.\n\nTry the following:\n- Retry the query, as transient network errors often resolve after a retry.\n- Add more specificity to the columns returned. For example, instead of `SELECT *`, try `SELECT col1, col2`.\n- If the issue repeats, contact your infrastructure team.\n\nThis message — DB::NetException (NETWORK_ERROR) — points to a low-level network write failure, not a query syntax, access, or storage issue.',
  },
  {
    code: 62,
    name: "SYNTAX_ERROR",
    regexp:
      "Syntax error: failed at position (?<position>\\d+) \\('(?<token>.+?)'\\) \\(line (?<line>\\d+), col (?<column>\\d+)\\): (?<context>(?:.|\\n|\\r+)+)\\. Expected one of: (?<expected_tokens>.+)",
    template:
      "This error indicates that the SQL query contains a syntax error at the specified position in the query text.\n\nCommon causes:\n- Misspelled keywords or function names.\n- Missing or misplaced punctuation (commas, parentheses, quotes).\n- Incorrect SQL structure or clause ordering.\n- Using reserved keywords without proper escaping.\n\nTry the following:\n- Review the query text around line {line}, column {column}.\n- Check the token '{token}' and ensure it's valid SQL syntax.\n- Verify that all parentheses, quotes, and brackets are properly matched.\n- Ensure proper spacing between SQL keywords and identifiers.\n- If using table or column names that are reserved words, escape them with backticks.\n\nThis is a query syntax issue that must be corrected before the query can execute.",
  },
  {
    code: 62,
    name: "SYNTAX_ERROR/2",
    regexp:
      "Could not apply macros: unexpected number of arguments: expected (?<expected_count>.+?), received (?<received_count>.+?)",
    template:
      "This error indicates that the SQL query contains a syntax error at macros.\n\nCommon causes:\n- Misspelled macros name.\n- Missing or misplaced punctuation (commas, parentheses, quotes).\n- Incorrect macros usage.\n\nTry the following:\n- Review the query text around the macros.\n- Verify that all macros' arguments are provided and properly separated.\n- Ensure proper spacing between SQL keywords and identifiers.\n\nThis is a query syntax issue that must be corrected before the query can execute.",
  },
  {
    code: 62,
    name: "SYNTAX_ERROR/3",
    regexp: "Could not apply macros: ",
    template:
      "This error indicates that the SQL query contains a syntax error at macros.\n\nCommon causes:\n- Misspelled macros name.\n- Missing or misplaced punctuation (commas, parentheses, quotes).\n- Incorrect macros usage.\n\nTry the following:\n- Review the query text around the macros.\n- Verify that all macros' arguments are provided and properly separated.\n- Ensure proper spacing between SQL keywords and identifiers.\n\nThis is a query syntax issue that must be corrected before the query can execute.",
  },
  {
    code: 497,
    name: "ACCESS_DENIED",
    regexp:
      "(?<username>\\S+?): Not enough privileges\\. To execute this query, it's necessary to have the grant (?<required_permissions>.+?) ON (?<database>.\\w+?)\\.(?<table>\\w+)",
    template:
      "This error indicates that the user (`{username}`) does not have sufficient permissions to execute the query on the specified database object.\n\n- Required permissions: {required_permissions}\n- Target resource: `{database}.{table}`\n\nTry the following:\n- Verify that you are using the correct user credentials for `{username}`.\n- Check if you have access to the `{database}` database and `{table}` table.\n- Ensure that the database and table names in your query are correct.\n- Contact your Hydrolix representative to request the necessary permissions.\n\nThis is a security/permissions issue, not a syntax or connectivity problem.",
  },
  {
    code: 394,
    name: "QUERY_WAS_CANCELLED",
    regexp:
      "Query was cancelled or a client has unexpectedly dropped the connection",
    template:
      "This error indicates that the query execution was interrupted, either by an explicit cancellation or because the client connection was lost.\n\nPossible causes:\n- User manually cancelled the query.\n- Client application (e.g., Grafana, clickhouse-client) disconnected before query completion.\n- Network timeout between client and server.\n- Client application crashed or was terminated.\n\nTry the following:\n- Retry the query if it was cancelled unintentionally.\n- Check network stability between client and server.\n- Increase timeout settings in your client application if queries are timing out.\n- Review client application logs for unexpected disconnections.\n\nThis is typically a client-side or network issue, not a problem with the query itself.",
  },
  {
    code: 46,
    name: "UNKNOWN_FUNCTION",
    regexp: "Unknown function (?<function_name>\\S+)",
    template:
      "This error indicates that the query uses a function name that is not recognized by the database system.\n\nCommon causes:\n- Misspelled function name.\n- The function is not available in this version of the database.\n- Using a function from a different database system (e.g., a PostgreSQL function in ClickHouse).\n\nTry the following:\n- Check the spelling of '{function_name}' in your query.\n- Verify that this function is supported in your database version.\n- Consult the database documentation for available functions and correct syntax.\n\nThis is a syntax/compatibility issue that requires correcting the function name or using an alternative approach.",
  },
  {
    code: 47,
    name: "UNKNOWN_IDENTIFIER",
    regexp:
      "Missing columns: '(?<identifier_name>.+?)' while processing query:",
    template:
      "This error indicates that the query references a column or identifier that does not exist in the queried table(s).\n\nCommon causes:\n- Misspelled column name.\n- The column does not exist in the specified table.\n- Missing a table alias or providing an incorrect table reference.\n- The column was removed or renamed in the schema.\n\nTry the following:\n- Verify the spelling of '{identifier_name}'.\n- Check that this column exists in the table using `DESCRIBE {{table_name}}` or a similar command.\n- Ensure you're querying the correct table.\n- If the column name contains special characters, ensure it's properly escaped.\n- Review your table schema to confirm available columns.\n\nThis is a schema/reference issue that requires using the correct column name.",
  },
  {
    code: 60,
    name: "UNKNOWN_TABLE",
    regexp: "Table (?<table_name>.+?) does not exist",
    template:
      "This error indicates that the query references a table that does not exist in the database.\n\nCommon causes:\n- Misspelled table name.\n- The table does not exist in the current database.\n- Missing a database qualifier (e.g., it should be `database.table`).\n- The table was deleted or not yet created.\n- Querying the wrong database or cluster.\n\nTry the following:\n- Verify the spelling and case of '{table_name}'.\n- List available tables using `SHOW TABLES` to confirm the table exists.\n- Ensure you're connected to the correct database.\n- If the table is in a different database, use the fully qualified name: `database.table`.\n- Check with your administrator if the table should exist.\n\nThis is a schema/reference issue that requires using the correct table name or creating the table.",
  },
  {
    code: 159,
    name: "TIMEOUT_EXCEEDED",
    regexp:
      "Timeout exceeded: elapsed (?<elapsed_time>.+?) seconds, maximum: (?<max_timeout>\\d+)",
    template:
      "This error indicates that the query execution time exceeded the configured timeout limit.\n\n- Query execution time: {elapsed_time} seconds\n- Maximum allowed: {max_timeout} seconds\n\nCommon causes:\n- Query is too complex or processes too much data.\n- Insufficient system resources (CPU, memory, I/O).\n\nTry the following:\n- Optimize your query by adding filters to reduce data processed.\n- Break complex queries into smaller, simpler queries.\n- Increase the timeout setting if you have permission (`hdx_query_max_execution_time`).\n- Consider reducing the time range of your query.\n- Use LIMIT clause to restrict result size.\n- Contact your Hydrolix representative to seek help with optimizing the query.\n\nThis is a performance issue that may require query optimization or configuration changes.",
  },
  {
    code: 53,
    name: "TYPE_MISMATCH",
    regexp:
      "Cannot convert (?:.+?) '(?:.+?)' to type (?:.+?): While processing (?:.+)",
    template:
      "This error indicates that there is a data type incompatibility in the query.\n\nCommon causes:\n- Comparing or combining values of incompatible types.\n- Function arguments have incorrect types.\n- Performing arithmetic operations on non-numeric types.\n\nTry the following:\n- Review the data types of columns and expressions in your query.\n- Use explicit type conversion functions (`CAST`, `toInt32`, `toString`, etc.).\n- Ensure function arguments match the expected types.\n- Check that comparison operators are used with compatible types.\n\nThis is a type compatibility issue that requires proper type casting or using compatible data types.",
  },
  {
    code: 209,
    name: "SOCKET_TIMEOUT",
    regexp:
      "Timeout exceeded while (?:.+?) to socket \\((?:.+?), (?:.+?) ms\\)",
    template:
      "This error indicates that a network socket operation timed out while communicating with the database server.\n\nCommon causes:\n- Network latency or instability between the client and server.\n- The query is taking longer than the socket timeout setting.\n- A firewall or network device is dropping connections.\n\nTry the following:\n- Retry the query, as this may be a transient network issue.\n- Check network connectivity and latency to the server.\n- Increase socket timeout settings if possible.\n- Optimize query performance to reduce execution time.\n- Contact your network administrator if the issue persists.\n\nThis is a network/performance issue, not a problem with the query syntax.",
  },
  {
    code: 427,
    name: "CANNOT_COMPILE_REGEXP",
    regexp:
      "OptimizedRegularExpression: cannot compile re2: (?<pattern>.+?), error: (?:.+)",
    template:
      "This error indicates that a regular expression pattern provided in the query is invalid or cannot be compiled.\n\nInvalid pattern: {pattern}\n\nCommon causes:\n- Invalid regex syntax (e.g., unclosed brackets, invalid escape sequences).\n- Unsupported regex features for this database system.\n- Special characters are not properly escaped.\n- Incomplete or malformed pattern.\n\nTry the following:\n- Review the regex pattern '{pattern}' for syntax errors.\n- Test your regex pattern using an online regex tester.\n- Ensure special characters are properly escaped.\n- Consult the database documentation for supported regex syntax.\n- Simplify complex patterns or break them into multiple conditions.\n\nThis is a regex syntax issue that requires correcting the pattern.",
  },
  {
    code: 1001,
    name: "TIMERANGE_REQUIRED",
    regexp:
      "<HdxStorageError hdx_query_timerange_required is set to true\\. Your query needs a time range filter in a WHERE clause>",
    template:
      "This error indicates that the query must include a time range filter, but none was provided.\n\nThe database configuration requires all queries to specify a time window to improve performance and resource management.\n\nTry the following:\n- Add a `WHERE` clause with time constraints using timestamp columns.\n- Example: `WHERE timestamp >= toDateTime('{{start_time}}') AND timestamp <= toDateTime('{{end_time}}')`\n- Ensure the time filter covers a reasonable range (not too broad).\n- Check which column is designated as the time column for your table.\n\nThis is a query constraint issue that requires adding proper time filters to your query.",
  },
  {
    code: 1001,
    name: "PARTITION_IO_TIMEOUT",
    regexp:
      "<HdxBadPartitionError error reading partition=(?<partition_id>.+?) io timed out after (?<timeout_ms>.+?)ms>",
    template:
      "This error indicates that reading data from a specific partition timed out after {timeout_ms} milliseconds.\n\nAffected partition: {partition_id}\nTimeout (ms): {timeout_ms}\n\nCommon causes:\n- Network issues accessing remote storage (e.g., S3, Azure).\n- The storage backend is overloaded or throttled.\n- A large partition is requiring more time to read.\n\nTry the following:\n- Retry the query, as this may be a transient I/O issue.\n- Contact your infrastructure team if the issue persists.\n\nThis is a storage I/O issue, not a query syntax problem.",
  },
  {
    code: 1001,
    name: "PARTITION_RATE_LIMIT",
    regexp:
      '<HdxBadPartitionError error reading partition=(?<partition_id>.+?) error=request_failed status_code=(?<http_status_code>.+?) path=(?<storage_path>.+?) <\\?xml version=""1\\.0"" encoding=""UTF-8""\\?><Error><Code>SlowDown</Code><Message>Please reduce your request rate\\.</Message><RequestId></RequestId></Error>>',
    template:
      "This error indicates that cloud storage is rate-limiting requests.\n\n- HTTP Status: `{http_status_code}`\n- Affected partition: `{partition_id}`\n- Storage path: `{storage_path}`\n\nThis typically occurs with cloud storage providers (AWS S3, Azure Blob, GCS) when request rates exceed their limits.\n\nTry the following:\n- Retry the query after a brief delay; rate limits are usually temporary.\n- Reduce query concurrency or frequency if running many queries.\n- Contact your cloud provider or Hydrolix representative to review and/or adjust rate limits.\n\nThis is a cloud storage rate limiting issue, not a query problem.",
  },
  {
    code: 1001,
    name: "NAMESPACE_NOT_EXIST",
    regexp:
      "<DatabaseError namespace='(?<namespace>.+?)' does not exist in db='(?<database>.+?)'>",
    template:
      "This error indicates that the specified namespace does not exist in the database.\n\nNamespace: {namespace}\nDatabase: {database}\n\nTry the following:\n- Verify the namespace name spelling and case.\n- List available namespaces using `SHOW DATABASES` or an equivalent command.\n- Ensure you have access to the correct database.\n- Check with your administrator if the namespace should exist.\n\nThis is a schema/reference issue that requires using the correct namespace or creating it.",
  },
  {
    code: 1001,
    name: "DATABASE_NOT_EXIST",
    regexp: "<ContextError db='(?<database>.+?)' does not exist>",
    template:
      "This error indicates that the specified database does not exist.\n\nDatabase: {database}\n\nTry the following:\n- Verify the database name spelling and case.\n- List available databases using `SHOW DATABASES` or an equivalent command.\n- Ensure you have access to the correct database.\n- Check with your administrator if the database should exist.\n\nThis is a schema/reference issue that requires using the correct database or creating it.",
  },
  {
    code: 1001,
    name: "POOL_NOT_EXIST",
    regexp: "<ClusterError Pool name (?<pool_name>.+?) does not exist>",
    template:
      "This error indicates that the specified query pool does not exist.\n\nPool name: {pool_name}\n\nQuery pools are used to manage and isolate workloads in the cluster.\n\nTry the following:\n- Verify the pool name spelling and case.\n- Check with your infrastructure team about correct pool names.\n- Ensure you have access to the specified pool.\n- Use the default pool if no specific pool is required.\n\nThis is a cluster configuration issue that requires using a valid pool name.",
  },
  {
    code: 1001,
    name: "MEMORY_ALLOCATION",
    regexp:
      "Received from (?<server>.+?)\\. DB::Exception: std::bad_alloc: std::bad_alloc\\.: While executing HdxPeerSource",
    template:
      "This error indicates that the system could not allocate sufficient memory to execute the query.\n\nThis is a critical system resource issue.\n\nCommon causes:\n- The query requires more memory than is available.\n- The system is under memory pressure from other processes.\n- A memory leak or excessive memory consumption.\n- The query is processing very large datasets without proper limits.\n\nTry the following:\n- Reduce query complexity or add filters to process less data.\n- Use a `LIMIT` clause to restrict the result size.\n- Break large queries into smaller chunks.\n- Contact your system administrator for resource allocation.\n\nThis is a critical system resource issue that requires query optimization or infrastructure changes.",
  },
  {
    code: 1001,
    name: "DISK_SPACE_EXCEEDED",
    regexp:
      "Received from (?<server>.+?)\\. DB::Exception: h::db::HdxBadPartitionError: <HdxBadPartitionError error reading partition=(?<partition_id>.+?) disk space used%=(?<used_percentage>.+?) > redzone=(?<redzone_percentage>.+?)% configured limit for mountpoint \\.\\.\\.",
    template:
      "This error indicates that the disk space usage has exceeded the configured safety threshold (redzone).\n\nServer: {server}\nAffected partition: {partition_id}\nCurrent disk usage: {used_percentage}%\nRedzone threshold: {redzone_percentage}%\n\nThis is a critical system resource issue that requires immediate attention.\n\nCommon causes:\n- The disk is filling up with data, logs, or temporary files.\n- Insufficient disk capacity for the workload.\n\nTry the following:\n- Contact your system administrator immediately.\n- Pause non-critical queries until space is freed.\n- Use a dedicated pool for complex queries.\n- Optimize the query (e.g., time range filter, joins, selected data).\n\nThis is a critical system resource issue that requires query optimization or infrastructure changes.",
  },
  {
    code: 1001,
    name: "FILESYSTEM_ERROR",
    regexp:
      "Received from (?<server>.+?)\\. DB::Exception: std::__1::__fs::filesystem::filesystem_error: filesystem error: in canonical: No such file or directory \\[(?<partition_id>.+?)\\]",
    template:
      "This error indicates a filesystem-level error, typically when trying to access a file or directory that doesn't exist.\n\nServer: {server}\nAffected partition: {partition_id}\n\nCommon causes:\n- Incorrect file path configuration.\n- A permissions issue is preventing access.\n- A corrupted filesystem or storage issue.\n\nTry the following:\n- Retry the query in case this is a transient issue.\n- Contact your infrastructure team to investigate it.\n\nThis is an issue that requires investigation by the infrastructure team.",
  },
  {
    code: 1001,
    name: "TEMP_FILE_CREATION",
    regexp:
      "Received from (?<server>.+?)\\. DB::Exception: h::db::HdxBadPartitionError: <HdxBadPartitionError error reading partition=(?<partition_id>.+?) failed to create tmp file in (?<temp_path>.+?) - No such file or directory>",
    template:
      "This error indicates that the system failed to create a temporary file needed for query processing.\n\nServer: {server}\nTemp file location: {temp_path}\nAffected partition: {partition_id}\n\nCommon causes:\n- Too many parallel queries are running.\n- The temporary directory doesn't exist or is misconfigured.\n\nTry the following:\n- Retry the query in case this is a transient issue.\n- Use another query pool.\n- Optimize query performance and the amount of processed data.\n- Contact your infrastructure team to review the configuration.\n\nThis is an issue that requires query optimization or investigation by the infrastructure team if it constantly repeats.",
  },
  {
    code: 1001,
    name: "PARTITION_DNS_RESOLUTION",
    regexp:
      "<HdxBadPartitionError error reading partition=(?<partition_id>.+?) getaddrinfo failed to resolve '(?<hostname>.+?)'\\. Name or service not known>",
    template:
      "This error indicates that the system could not resolve the hostname for accessing partition data.\n\nHostname: {hostname}\nAffected partition: {partition_id}\n\nCommon causes:\n- An infrastructure change caused some hostnames to become unavailable.\n- Network connectivity issues.\n\nTry the following:\n- Retry the query in case this is a transient issue.\n- Contact your infrastructure team if DNS issues persist.\n\nThis is an infrastructure issue and should be investigated by the infrastructure team.",
  },
  {
    code: 1001,
    name: "PARTITION_SSL_READ_ERROR",
    regexp:
      "<HdxBadPartitionError error reading partition=(?<partition_id>.+?) SSL_read failed: (?<ssl_error>.+?) eof while reading>",
    template:
      "This error indicates a failure while reading data over an SSL/TLS connection.\n\nSSL read error: {ssl_error}\n\nCommon causes:\n- SSL certificate issues (e.g., expired, invalid, untrusted).\n- SSL/TLS version mismatch or cipher incompatibility.\n- The connection was interrupted during the SSL handshake.\n- A firewall or proxy is interfering with SSL traffic.\n- Storage backend SSL configuration changes.\n\nTry the following:\n- Retry the query, as this may be a transient connection issue.\n- Contact your infrastructure team to review the SSL configuration.\n\nThis is not a query issue and should be reviewed by the infrastructure team.",
  },
  {
    code: 1001,
    name: "PARTITION_SSL_CONNECT_ERROR",
    regexp:
      "<HdxBadPartitionError error reading partition=(?<partition_id>.+?) SSL_connect failed: (?<ssl_error>.+?)>",
    template:
      "This error indicates a failure while establishing an SSL/TLS connection to the storage backend.\n\nAffected partition: {partition_id}\nStorage path: {ssl_error}\n\nCommon causes:\n- Cannot establish an SSL handshake with the storage backend.\n- SSL certificate validation failure.\n- The network is blocking SSL connections.\n- Storage endpoint SSL configuration issues.\n- Incompatible SSL/TLS protocols or ciphers.\n\nTry the following:\n- Retry the query, as this may be a transient connection issue.\n- Contact your infrastructure team to review the SSL configuration.\n\nThis is not a query issue and should be reviewed by the infrastructure team.",
  },
  {
    code: 1001,
    name: "PARTITION_SERVER_BUSY",
    regexp:
      '<HdxBadPartitionError error reading partition=(?<partition_id>.+?) error=request_failed status_code=(?<http_status_code>.+?) path=(?<storage_path>.+?) <\\?xml version=""1\\.0"" encoding=""utf-8""\\?><Error><Code>ServerBusy</Code><Message>Egress is over the account limit \\.\\.\\.',
    template:
      "This error indicates that the storage backend server is too busy to handle the request.\n\nHTTP Status: {http_status_code}\nAffected partition: {partition_id}\nStorage path: {storage_path}\n\nCommon causes:\n- The storage backend is overloaded with requests.\n- Temporary resource constraints on the storage service.\n- High concurrent query load.\n- Storage service scaling or maintenance activities.\n\nTry the following:\n- Retry the query after a brief delay; this is often temporary.\n- Reduce concurrent query load if possible.\n- Contact your infrastructure team if the issue persists.\n\nThis is not a query issue, and the infrastructure team should review it if the error persists.",
  },
  {
    code: 1001,
    name: "PARTITION_AUTH_FAILED",
    regexp:
      '<HdxBadPartitionError error reading partition=(?<partition_id>.+?) error=request_failed status_code=(?<http_status_code>.+?) path=(?<storage_path>.+?) <\\?xml version=""1\\.0"" encoding=""utf-8""\\?><Error><Code>AuthenticationFailed</Code><Message>Server failed to authenticate the request\\. Make sure the value of Authorization header is formed correctly including the signature',
    template:
      "This error indicates that authentication to the storage backend failed.\n\nHTTP Status: {http_status_code}\nAffected partition: {partition_id}\nStorage path: {storage_path}\n\nCommon causes:\n- Credentials are not properly configured.\n- Permissions were revoked or changed on the storage backend.\n- Storage account configuration changes.\n\nTry the following:\n- Retry the query, as this may be a transient issue.\n- Contact your infrastructure team to review the storage access configuration.\n\nThis is not a query issue, and the infrastructure team should review it if the error persists.",
  },
  {
    code: 1001,
    name: "PARTITION_READ_ERROR",
    regexp:
      "<partition read error> partition=(?<partition_id>.+?) path=(?<storage_path>.+?)",
    template:
      "This error indicates a general failure while reading data from a partition.\n\nAffected partition: {partition_id}\nStorage path: {storage_path}\n\nCommon causes:\n- Storage backend errors or failures.\n- Network issues during data transfer.\n- Insufficient permissions to read the partition.\n- Partial or incomplete partition files.\n\nTry the following:\n- Retry the query to see if the issue is transient.\n- Contact your infrastructure team to investigate partition integrity.\n\nThis is not a query issue, and the infrastructure team should review it if the error persists.",
  },
  {
    code: 1001,
    name: "CATALOG_CONNECTION_LOST",
    regexp:
      "<CatalogError Failed to submit transaction: Lost connection to the database server\\. \\(after (?<retry_count>.+?) retries\\)>",
    template:
      "This error indicates that the connection to the metadata catalog was lost.\n\nRetry attempts: {retry_count}\n\nThe catalog stores metadata about databases, tables, and partitions.\n\nCommon causes:\n- The catalog database is unavailable or has crashed.\n- Network connectivity issues to the catalog database.\n- The catalog database is overloaded or unresponsive.\n\nTry the following:\n- Retry the query to see if the issue is transient.\n- Contact your infrastructure team to investigate catalog health.\n\nThis is a catalog database connectivity issue that requires infrastructure investigation.",
  },
  {
    code: 1001,
    name: "CATALOG_LOOKUP_UNSUPPORTED",
    regexp:
      "<HdxStorageError Catalog look-up not supported at query-peer level>",
    template:
      "This error indicates that a catalog lookup operation is not supported in the current configuration.\n\nCommon causes:\n- Attempting an unsupported metadata operation.\n\nTry the following:\n- Retry the query to see if the issue is transient.\n- Contact your infrastructure team to investigate it.\n\nThis is a catalog database connectivity issue that requires infrastructure investigation.",
  },
  {
    code: 1001,
    name: "CATALOG_TRANSACTION_ABORTED",
    regexp:
      "<CatalogError Failed to submit transaction: ERROR:  current transaction is aborted, commands ignored until end of transaction block>",
    template:
      "This error indicates that the catalog database transaction was aborted and no further commands can be executed in this transaction.\n\nThis typically occurs with catalog databases when a previous error caused a transaction rollback.\n\nCommon causes:\n- A deadlock or lock timeout in the catalog database due to interfering queries.\n- A connection interruption during a transaction.\n\nTry the following:\n- Retry the query to see if the issue is transient.\n- Contact your infrastructure team to investigate it.\n\nThis is a catalog database transaction issue that typically resolves on retry.",
  },
  {
    code: 1001,
    name: "INVALID_REGEX",
    regexp: "<RegexError invalid regex provided (?<pattern>.+?)>",
    template:
      "This error indicates that an invalid regular expression pattern was provided.\n\nInvalid pattern: {pattern}\n\nCommon causes:\n- A regex syntax error (e.g., unclosed brackets, invalid escapes).\n- The pattern uses unsupported regex features.\n- Special characters are not properly escaped.\n\nTry the following:\n- Review the regex pattern '{pattern}' for syntax errors.\n- Test the pattern using a regex validator tool.\n- Ensure special regex characters are properly escaped.\n- Consult the documentation for supported regex syntax.\n- Simplify complex patterns or use alternative approaches.\n\nThis is a regex validation issue that requires correcting the pattern.",
  },
  {
    code: 6,
    name: "CANNOT_PARSE_TEXT",
    regexp:
      "The text does not contain '(?<character>.+?)': (?<text_value>\\S+)",
    template:
      "This error indicates that the system could not parse text data because it was missing an expected character or format.\n\nMissing character: '{character}'\nText value: '{text_value}'\n\nCommon causes:\n- A variable or macro in the query (e.g., `${{myvar:text}}`) was not substituted with a valid value, leaving the raw macro in the query text.\n- The input data has an unexpected format.\n- Invalid characters in the input data.\n\nTry the following:\n- Ensure that any variables or macros used in the query are correctly replaced with their intended values before execution.\n- Review the query for correctness.\n\nThis is a query issue that requires fixing the SQL syntax or the variable substitution mechanism.",
  },
  {
    code: 10,
    name: "NOT_FOUND_COLUMN_IN_BLOCK",
    regexp: "Not found column (?<column_name>.+) in block",
    template:
      "This error indicates that a referenced column was not found in the data block being processed.\n\nColumn: {column_name}\n\nCommon causes:\n- The column name is misspelled or has an incorrect case.\n- The column doesn't exist in the queried table.\n- The column was dropped from the schema.\n- A subquery or join doesn't produce the expected columns.\n- An alias resolution issue.\n\nTry the following:\n- Verify the spelling and case of '{column_name}'.\n- Use `DESCRIBE {{table_name}}` to list available columns.\n- Check that subquery `SELECT` lists include the required columns.\n- Ensure `JOINs` properly reference columns from the correct tables.\n- Use fully qualified names (`table.column`) to avoid ambiguity.\n\nThis is a column reference issue that requires using correct column names.",
  },
  {
    code: 26,
    name: "CANNOT_PARSE_QUOTED_STRING",
    regexp:
      "Cannot parse quoted string: (?:.+?): while converting '(?:.+?)' to (?:.+)",
    template:
      "This error indicates that the system could not parse a quoted string value, often when trying to interpret it as a specific data type like an Array.\n\nCommon causes:\n- Unmatched quotes in a string literal.\n- Invalid escape sequences within the string.\n- The string contains unsupported special characters for the target type.\n- Using double quotes (`\"`) where single quotes (`'`) are expected for array elements.\n\nTry the following:\n- Check that all string quotes are properly matched and closed.\n- Use proper escape sequences for special characters (e.g., `\\\\`, `\\'`).\n- When defining arrays as strings, ensure array elements are enclosed in single quotes, e.g., `['allow']`.\n- Check for embedded quotes that need escaping.\n\nThis is a parsing issue with a string constant in the query that requires proper quoting and escaping.",
  },
  {
    code: 27,
    name: "CANNOT_PARSE_INPUT_ASSERTION_FAILED",
    regexp:
      "Cannot parse input: expected (?<expected_character>.+?) at end of stream\\.: \\(at row (?<row>.+?)\\)",
    template:
      "This error indicates that the parser encountered an unexpected end of input or a formatting error while reading a data file, such as a dictionary.\n\nExpected: '{expected_character}'\nRow: {row}\n\nThis typically points to a malformed truncated query.\n\nCommon causes:\n- Corrupted or truncated query.\n- Incompatible query format.\n\nTry the following:\n- Retry the query to see if the issue is transient.\n- Validate query structure and format.\n- Validate query isn't too long.\n- Contact support if this appears to be a parsing bug.\n\nThis is a query format or syntax issue that requires the query optimization or refactoring.",
  },
  {
    code: 32,
    name: "ATTEMPT_TO_READ_AFTER_EOF",
    regexp:
      "Attempt to read after eof: while receiving packet from (?<server>\\S+)",
    template:
      "This error indicates that the client attempted to read data from the server, but the connection was already closed by the server (End Of File).\n\nServer: {server}\n\nCommon causes:\n- The server closed the stream due to an internal error, timeout, or because the query was cancelled.\n- Network connectivity issues that caused an abrupt connection termination.\n\nTry the following:\n- Retry the query to see if the issue is transient.\n- Check for other server-side errors that might have occurred and caused the connection to drop.\n- Simplify the query to see if a specific part is causing the server to terminate the connection.\n\nThis error often points to a server-side problem or a network interruption.",
  },
  {
    code: 33,
    name: "CANNOT_READ_ALL_DATA",
    regexp:
      "Cannot read all data\\. Bytes read: (?<bytes_read>.+?)\\. Bytes expected: (?<bytes_expected>.+?)\\.: while receiving packet from (?<source>\\S+)",
    template:
      "This error indicates that the client could not read all the expected data from a source, suggesting that the data transfer was incomplete.\n\nSource: {source}\nDetails: Read {bytes_read} out of {bytes_expected} bytes.\n\nCommon causes:\n- Network interruption during data transfer.\n- Timeout while reading from the source.\n- The server closed the connection prematurely.\n\nTry the following:\n- Retry the operation to see if it's a transient issue.\n- Check network connectivity and stability.\n\nThis is a data transfer issue that may be transient.",
  },
  {
    code: 36,
    name: "BAD_ARGUMENTS",
    regexp:
      "Element of set in IN, VALUES, or LIMIT, or aggregate function parameter, or a table function argument is not a constant expression",
    template:
      "This error indicates that a non-constant or invalid argument was provided to an operation that requires a constant value, such as an empty tuple in an `IN` clause.\n\nCommon causes:\n- Using an empty tuple `()` in an `IN` clause (e.g., `myvar IN tuple()`).\n- A function was called with the wrong number of arguments or invalid types.\n- Argument values are outside the valid range.\n\nTry the following:\n- Review function documentation for correct argument usage.\n- Ensure that clauses like `IN` are not passed empty sets. Your application logic should handle cases where the set of values is empty and avoid generating such a query.\n- Verify that argument values are within valid ranges and of compatible types.\n\nThis is an argument validation issue in the query that requires using correct and valid function or clause parameters.",
  },
  {
    code: 42,
    name: "NUMBER_OF_ARGUMENTS_DOESNT_MATCH",
    regexp:
      "Number of arguments for function (?<function_name>.+?) doesn't match: passed (?<provided_count>\\d+), should be (?<expected_count>\\d+)",
    template:
      "This error indicates that a function was called with the wrong number of arguments.\n\nFunction: {function_name}\nProvided: {provided_count} arguments\nExpected: {expected_count} arguments\n\nTry the following:\n- Review the documentation for the function '{function_name}'.\n- Check the function signature for required and optional parameters.\n- Add any missing required arguments.\n- Remove any extra arguments.\n- Verify you're using the correct function overload if applicable.\n\nThis is a query's function call issue that requires using the correct number of arguments.",
  },
  {
    code: 42,
    name: "NUMBER_OF_ARGUMENTS_DOESNT_MATCH/2",
    regexp:
      "An incorrect number of arguments was specified for function (?<function_name>.+?)\\. Expected (?<expected_count>.+?), got (?<provided_count>.+?) arguments: While processing (?<expression>.+?)",
    template:
      "This error indicates that a function was called with the wrong number of arguments.\n\nFunction: {function_name}\nProvided: {provided_count}\nExpected: {expected_count}\n\nTry the following:\n- Review the documentation for the function '{function_name}'.\n- Check the function signature for required and optional parameters.\n- Add any missing required arguments.\n- Remove any extra arguments.\n- Verify you're using the correct function overload if applicable.\n\nThis is a query's function call issue that requires using the correct number of arguments.",
  },
  {
    code: 43,
    name: "ILLEGAL_TYPE_OF_ARGUMENT",
    regexp:
      "Illegal type (?<type>.+?) of function (?<function>.+?): While processing (?<expression>.+)",
    template:
      "This error indicates that a function argument has an incompatible type. For example, placing a non-nullable type inside a nullable container when it's not supported.\n\nExpression: {expression}\nProvided nested type: {type}\nFunction: {function}\n\nTry the following:\n- Use `CAST()` to convert the argument to the expected type.\n- Use type-specific functions (e.g., `toInt32`, `toString`, `toDateTime`).\n- Verify the data types of columns used as arguments.\n- Check the function documentation for accepted types.\n- Consider using a different function that accepts your data type.\n\nThis is a function argument type compatibility issue in the query that requires type conversion or using compatible types.",
  },
  {
    code: 43,
    name: "ILLEGAL_TYPE_OF_ARGUMENT/2",
    regexp:
      "Nested type (?<nested_type>.+?) cannot be inside (?<container_type>.+?) type: While processing (?<expression>.+)",
    template:
      "This error indicates that a function argument has an incompatible nested type. For example, placing a non-nullable type inside a nullable container when it's not supported.\n\nExpression: {expression}\nProvided nested type: {nested_type}\nContainer type: {container_type}\n\nTry the following:\n- Use `CAST()` to convert the argument to the expected type.\n- Use type-specific functions (e.g., `toInt32`, `toString`, `toDateTime`).\n- Verify the data types of columns used as arguments.\n- Check the function documentation for accepted types.\n- Consider using a different function that accepts your data type.\n\nThis is a function argument type compatibility issue in the query that requires type conversion or using compatible types.",
  },
  {
    code: 70,
    name: "CANNOT_CONVERT_TYPE",
    regexp: "Cannot convert (?<from_type>\\w+) to (?<to_type>\\w+)",
    template:
      "This error indicates that a type conversion is not possible or has failed.\n\nFrom type: {from_type}\nTo type: {to_type}\n\nCommon causes:\n- The value cannot be represented in the target type (e.g., converting a non-numeric string to a number).\n- The conversion would unacceptably lose data or precision.\n- The string value cannot be parsed as the target type.\n- Type conversion is not supported between these types.\n\nTry the following:\n- Verify the value can be validly converted to the target type.\n- Check for invalid data values that cannot be converted.\n- Consider alternative type conversions or transformations.\n- Validate and clean the data before conversion.\n\nThis is a type conversion issue that requires valid data or alternative approaches.",
  },
  {
    code: 99,
    name: "UNKNOWN_PACKET_FROM_CLIENT",
    regexp: "Unknown packet from client (\\S+)",
    template:
      "This error indicates that the server received an unrecognized packet from the client.\n\nCommon causes:\n- Corrupted network data.\n- Network interference or packet corruption.\n\nTry the following:\n- Retry the query, as transient network errors often resolve after a retry.\n- Check for network connectivity issues that could be causing packet corruption.\n- Contact your administrator to verify protocol compatibility.\n\nThis is a network connectivity issue.",
  },
  {
    code: 117,
    name: "INCORRECT_DATA",
    regexp:
      "Received from (?<server>.+?)\\. DB::Exception: Failed to (?<operation>.+?) input '(?<input_data>.+?)': while executing 'FUNCTION (?<function_call>.+)'",
    template:
      "This error indicates that data is malformed, corrupted, or doesn't meet the validation requirements for a specific function (e.g., `base64Decode`).\n\nServer: {server}\nOperation: {operation}\nInput: '{input_data}'\n\nCommon causes:\n- Invalid data format or structure (e.g., a string is not valid Base64).\n- Encoding or character set issues.\n- Incomplete or truncated data.\n\nTry the following:\n- Validate the source data's quality and integrity.\n- Verify data encoding and character sets.\n- Ensure the data being passed to the function `{function_call}` is in the correct format.\n- Consider data repair or re-ingestion if it is corrupted.\n\nThis is a query issue that requires analysis of the data being processed.",
  },
  {
    code: 156,
    name: "DICTIONARIES_WAS_NOT_LOADED",
    regexp: "dictionaries not loaded, yet",
    template:
      "This error indicates that required dictionaries were not successfully loaded.\n\nDictionaries are used for lookup tables and enrichment in queries.\n\nCommon causes:\n- The dictionary is too large and there is insufficient memory.\n- Network issues accessing dictionary source or source is unavailable.\n- Dictionary configuration errors.\n\nTry the following:\n- Ensure dictionary size is not too large and there is sufficient memory\n- Check dictionary configuration in system tables.\n- Verify dictionary source (database, file, HTTP) is accessible.\n- Contact your Hydrolix representative to seek help with reloading or fixing dictionaries.\n\nThis is a dictionary loading issue that requires verifying dictionary sources and configuration.",
  },
  {
    code: 179,
    name: "MULTIPLE_EXPRESSIONS_FOR_ALIAS",
    regexp: "Different expressions with the same alias (?<alias_name>.+?):",
    template:
      "This error indicates that the same alias is defined multiple times with different expressions in the same query context.\n\nAlias: {alias_name}\n\nCommon causes:\n- Duplicate column aliases in the `SELECT` clause.\n- The same alias being used in multiple subqueries or CTEs that are being joined.\n- Ambiguous alias definitions.\n\nTry the following:\n- Review the query for the duplicate alias '{alias_name}'.\n- Rename one of the conflicting aliases to be unique.\n- Use fully qualified column names to avoid ambiguity.\n- Check subqueries and CTEs for alias conflicts.\n\nThis is a query alias definition issue that requires using unique alias names.",
  },
  {
    code: 184,
    name: "ILLEGAL_AGGREGATION",
    regexp:
      "Aggregate function (?<aggregate_function>.+?) is found in (?<clause>.+?) in query",
    template:
      "This error indicates an invalid use of an aggregate function, such as placing it in a `WHERE` clause.\n\nError details: {aggregate_function} found in {clause}.\n\nCommon causes:\n- Using an aggregate function (e.g., `SUM()`, `COUNT()`, `MIN()`) in a `WHERE` clause. `WHERE` filters rows before aggregation occurs.\n- Nesting aggregate functions (e.g., `SUM(COUNT(*))`).\n\nTry the following:\n- Move the condition on the aggregated result from the `WHERE` clause to a `HAVING` clause. `HAVING` is used to filter groups after aggregation.\n- To use the result of an aggregate function in a condition, you can also use a subquery.\n- Remove any nested aggregate functions; use subqueries to perform multi-level aggregations.\n\nThis is an aggregation syntax issue that requires the proper use of aggregate functions and `GROUP BY` or `HAVING` clauses.",
  },
  {
    code: 190,
    name: "SIZES_OF_ARRAYS_DONT_MATCH",
    regexp:
      "The argument (?:.+?) and argument (?:.+?) of function (?<function_name>.+?) have different array sizes",
    template:
      "This error indicates that an array operation (like `arrayZip`) requires arrays of the same size but received arrays with different lengths.\n\nFunction: {function_name}\n\nCommon causes:\n- Zipping or comparing arrays of different lengths.\n- Array operations that require equal-length inputs.\n- Data inconsistency that creates uneven arrays.\n\nTry the following:\n- Verify that the arrays being operated on should have the same length.\n- Use array functions to pad or truncate arrays to match sizes if appropriate.\n- Check the data generation logic to ensure array consistency.\n- Review the documentation for the array manipulation function being used to understand its requirements.\n\nThis is an array operation issue that requires matching array dimensions.",
  },
  {
    code: 215,
    name: "NOT_AN_AGGREGATE",
    regexp:
      "Column `(?<column_name>.+?)` is not under aggregate function and not in GROUP BY",
    template:
      "This error indicates that a column in the `SELECT` list is neither part of an aggregate function (like `SUM()`, `COUNT()`, `AVG()`) nor listed in the `GROUP BY` clause.\n\nColumn: `{column_name}`\n\nAccording to standard SQL rules, when a `GROUP BY` clause is used, any column in the `SELECT` list must be either used to group the rows (i.e., be in the `GROUP BY` list) or be used inside an aggregate function.\n\nTry the following:\n- Add the column `{column_name}` to your `GROUP BY` clause.\n- If you intend to perform an aggregation on `{column_name}`, wrap it in an aggregate function (e.g., `MAX({column_name})`, `groupArray({column_name})`).\n- Remove the column from the `SELECT` list if it's not needed.\n\nThis is a fundamental SQL aggregation issue that requires proper `GROUP BY` usage.",
  },
  {
    code: 347,
    name: "CANNOT_LOAD_CONFIG",
    regexp: "config not loaded, yet",
    template:
      "This error indicates that a configuration file could not be loaded.\n\nA common cause is that a critical system configuration file is inaccessible or doesn't exist.\n\nTo resolve the issue, contact your Hydrolix representative for assistance.\n\nThis is a configuration loading issue that requires verifying config files.",
  },
  {
    code: 349,
    name: "CANNOT_INSERT_NULL_IN_ORDINARY_COLUMN",
    regexp:
      "Cannot convert NULL value to non-Nullable type: while executing 'FUNCTION CAST\\((?<column_name>\\S+)",
    template:
      "This error indicates an attempt to insert or cast a `NULL` value into a column that is defined as non-nullable.\n\nColumn: {column_name}\n\nCommon causes:\n- A column is defined as `NOT NULL`, but an `INSERT` provides a `NULL` value.\n- A `SELECT` query attempts to `CAST` a column that contains `NULL`s to a non-nullable type.\n- The data source contains `NULL` values for a column that is non-nullable in the target table.\n\nTry the following:\n- Provide a non-NULL value for the column `{column_name}`.\n- Define a `DEFAULT` value for the column if appropriate.\n- Use `COALESCE` or `ifNull` to replace `NULL` with a default value before casting or inserting (e.g., `CAST(coalesce({column_name}, 'default_value'), '{{column_type}}')`.\n- Make the column nullable if `NULL` values are valid (`ALTER TABLE ... MODIFY COLUMN ... Nullable(...)`).\n\nThis is a `NULL` constraint violation that requires providing valid, non-null values.",
  },
  {
    code: 386,
    name: "NO_COMMON_TYPE",
    regexp: "There is no supertype for types (?<type1>\\w+), (?<type2>\\w+)",
    template:
      "This error indicates that an operation requires a common data type between different columns or expressions, but one could not be automatically determined.\n\nIncompatible types: {type1}, {type2}\n\nCommon causes:\n- `UNION` queries with incompatible column types between the `SELECT` statements.\n- `CASE`/`IF` expressions where different branches return incompatible types.\n- Array or tuple elements with incompatible types.\n\nTry the following:\n- Use explicit `CAST` to convert the types to a single, compatible type (e.g., `CAST(column as String)`).\n- Ensure that corresponding columns in a `UNION` have matching types.\n- Make sure that all branches of a `CASE` or `IF` statement return the same or compatible types.\n\nThis is a type compatibility issue that requires explicit type conversion.",
  },
  {
    code: 396,
    name: "TOO_MANY_ROWS_OR_BYTES",
    regexp:
      "Limit for result exceeded, max rows: (?<limit_value>.+?), current rows: (?<actual_value>.+)",
    template:
      "This error indicates that the query result exceeded the configured size limits.\n\nActual: {actual_value} rows\nLimit: {limit_value} rows\n\nCommon causes:\n- The query returns too many rows.\n- The result set is too large in bytes.\n- Missing or ineffective filters in the `WHERE` clause.\n- A `LIMIT` clause is missing or set too large.\n\nTry the following:\n- Add or adjust the `LIMIT` clause to restrict the row count.\n- Add more selective `WHERE` filters to reduce the result set.\n- Increase the result size limits if you have permission to do so.\n- Use pagination to retrieve results in chunks.\n- Aggregate or summarize the data instead of returning raw rows.\n\nThis is a result size issue that requires filtering, aggregation, or pagination.",
  },
  {
    code: -1,
    name: "UNKNOWN_ERROR",
    regexp: "Code: 1001, Unknown Error",
    template:
      "This catch-all error requires investigation to determine the root cause.\n\nTry the following:\n- Review the full error message for clues.\n- Check system logs for additional context.\n- Retry the operation to see if it's transient.\n- Simplify the query to isolate the issue.\n- Contact support with the complete error message.\n- Check for recent system changes or updates.\n\nThis requires investigation to determine the specific issue and resolution.",
  },
  {
    code: 59,
    name: "ILLEGAL_TYPE_OF_COLUMN_FOR_FILTER",
    regexp: "Invalid type for filter in (?<clause>.+?): (?<filter_type>.+?)",
    template:
      "This error indicates that the SQL query contains a filter in {clause} with invalid type {filter_type}.\n\nCommon causes:\n- Expression of filter should have a boolean value\n- Incorrect column used as a filter\n\nTry the following:\n- Review the query text around clause {clause}.\n- Check the filter type syntax and involved columns' types.\n- Verify that all parentheses, quotes, and brackets are properly matched.\n\nThis is a query issue that must be corrected before the query can execute.",
  },
];
