import { ErrorMessageBeautifier } from "./errorBeautifier";

describe("ErrorMessageBeautifier", () => {
  let beautifier: ErrorMessageBeautifier;

  beforeEach(() => {
    beautifier = new ErrorMessageBeautifier();
  });

  test("should return null if input string does not contain valid JSON", () => {
    const input = "Invalid string without JSON";
    const result = beautifier.beautify(input);
    expect(result).toBeUndefined();
  });

  test("should return null if parsed JSON does not contain an error property", () => {
    const input = 'prefix {"query": "SELECT *"} suffix';
    const result = beautifier.beautify(input);
    expect(result).toBeUndefined();
  });

  test("should extract default CH message", () => {
    const input =
      'prefix {"error": "Code: 123. DB::Exception: Something went wrong. (ABC)", "query": "SELECT * FROM table"} suffix';
    const result = beautifier.beautify(input);
    expect(result).toEqual("Something went wrong");
  });

  test("should extract default CH message multiline", () => {
    const input =
      'error querying the database: clickhouse [execute]:: 400 code: { "error": "Code: 62. DB::Exception: Syntax error: failed at position 238 (\'GROUP8\') (line 1, col 238): GROUP8 BY time ORDER BY time\\nSETTINGS hdx_query_max_execution_time=60, hdx_query_admin_comment=\'db=${__dashboard}; dp=Throughput; du=${__user.login}\'\\n. Expected one of: token, OpeningRoundBracket, FILTER, RESPECT NULLS, IGNORE NULLS, OVER, OR, AND, IS NOT DISTINCT FROM, IS NULL, IS NOT NULL, BETWEEN, NOT BETWEEN, LIKE, ILIKE, NOT LIKE, NOT ILIKE, REGEXP, IN, NOT IN, GLOBAL IN, GLOBAL NOT IN, MOD, DIV, alias, AS, GROUP BY, WITH, HAVING, WINDOW, ORDER BY, LIMIT, OFFSET, FETCH, SETTINGS, UNION, EXCEPT, INTERSECT, INTO OUTFILE, FORMAT, end of query. (SYNTAX_ERROR)", "query": "SELECT toStartOfInterval(toDateTime(reqTimeSec), INTERVAL 2 second) as time, sum(totalBytes) / sum(transferTimeMSec/1000) as throughput FROM akamai.logs WHERE reqTimeSec >= toDateTime(1740403350) AND reqTimeSec <= toDateTime(1740406950) GROUP8 BY time ORDER BY time\\nSETTINGS hdx_query_max_execution_time=60, hdx_query_admin_comment=\'db=${__dashboard}; dp=Throughput; du=${__user.login}\'\\n" }';
    const result = beautifier.beautify(input);
    expect(result).toEqual(
      "Syntax error: failed at position 238 ('GROUP8') (line 1, col 238): GROUP8 BY time ORDER BY time SETTINGS hdx_query_max_execution_time=60, hdx_query_admin_comment='db=${__dashboard}; dp=Throughput; du=${__user.login}' . Expected one of: token, OpeningRoundBracket, FILTER, RESPECT NULLS, IGNORE NULLS, OVER, OR, AND, IS NOT DISTINCT FROM, IS NULL, IS NOT NULL, BETWEEN, NOT BETWEEN, LIKE, ILIKE, NOT LIKE, NOT ILIKE, REGEXP, IN, NOT IN, GLOBAL IN, GLOBAL NOT IN, MOD, DIV, alias, AS, GROUP BY, WITH, HAVING, WINDOW, ORDER BY, LIMIT, OFFSET, FETCH, SETTINGS, UNION, EXCEPT, INTERSECT, INTO OUTFILE, FORMAT, end of query"
    );
  });

  test("should override message for code 159 with timeout", () => {
    const input =
      'prefix {"error": "Code: 159. DB::Exception: Timeout exceeded: elapsed 2.000005926 seconds, maximum: 2: While executing HdxSource.: While executing HdxPeerSource. (TIMEOUT_EXCEEDED)", "query": "SELECT 1"} suffix';
    const result = beautifier.beautify(input);
    expect(result).toEqual(
      "Timeout exceeded: elapsed 2.000005926 seconds, maximum: 2: While executing HdxSource.: While executing HdxPeerSource"
    );
  });

  test("should not override message for code 159 if timeout missed", () => {
    const input =
      'prefix {"error": "Code: 159. DB::Exception: Some error. (XYZ)", "query": "SELECT 1"} suffix';
    const result = beautifier.beautify(input);
    expect(result).toEqual("Some error");
  });

  test("should extract Hydrolix message", () => {
    const input =
      'prefix {"error": "<Error Hydrolix specific error message (Hydrolix v2)>", "query": "SELECT *"} suffix';
    const result = beautifier.beautify(input);
    expect(result).toEqual("Hydrolix specific error message");
  });

  test("should format singular unit when timeout equals 1", () => {
    const input =
      'prefix {"error": "Code: 159. DB::Exception: Some error. (XYZ) elapsed 1 seconds, maximum: 1", "query": "SELECT 1"} suffix';
    const result = beautifier.beautify(input);
    expect(result).toEqual("Some error.");
  });

  test("should format native connection refused", () => {
    const input = "dial tcp 172.232.16.177:9000: connect: connection refused";
    const result = beautifier.beautify(input);
    expect(result).toEqual("connect: connection refused");
  });

  test("should format native no such host", () => {
    const input =
      "dial tcp: lookup demo.trafficpeak.li on 127.0.0.11:53: no such host";
    const result = beautifier.beautify(input);
    expect(result).toEqual("no such host");
  });

  test("should format http connection refused", () => {
    const input =
      'Post "https://demo.trafficpeak.live:844?default_format=Native&hdx_query_output_format=Native&max_execution_time=14": dial tcp 172.232.16.177:844: connect: connection refused';
    const result = beautifier.beautify(input);
    expect(result).toEqual("connect: connection refused");
  });

  test("should format http no such host", () => {
    const input =
      'Post "https://demo.trafficpeak.l:844?default_format=Native&hdx_query_output_format=Native&max_execution_time=14": dial tcp: lookup demo.trafficpeak.l on 127.0.0.11:53: no such host';
    const result = beautifier.beautify(input);
    expect(result).toEqual("no such host");
  });

  test("should format auth error 400", () => {
    const input =
      'code: 516, message: Address: 10.2.15.40:56424 failed to authenticate user \'default\' due to <TurbineApiAuthenticatorError api login failed with provided username/password \'default\'. <HttpPermanentResponseError error=request_failed status_code=400 path=/config/v1/login {"username":["Enter a valid email address."],"password":["This field may not be blank."]} (Hydrolix v4.22.1 - Turbine dcb3e912)> (Hydrolix v4.22.1 - Turbine dcb3e912)>';
    const result = beautifier.beautify(input);
    expect(result).toEqual(
      "failed to authenticate user 'default':\n" +
        "username: Enter a valid email address.\n" +
        "password: This field may not be blank."
    );
  });

  test("should format auth error 401", () => {
    const input =
      "code: 516, message: Address: 10.2.15.40:35666 failed to authenticate user 'default@aa.aa' due to <TurbineApiAuthenticatorError api login failed with provided username/password 'default@aa.aa'. <HttpPermanentResponseError error=request_failed status_code=401 path=/config/v1/login {\"detail\":\"Could not login\"} (Hydrolix v4.22.1 - Turbine dcb3e912)> (Hydrolix v4.22.1 - Turbine dcb3e912)>";
    const result = beautifier.beautify(input);
    expect(result).toEqual(
      "failed to authenticate user 'default@aa.aa':\n" +
        "detail: Could not login"
    );
  });
});
