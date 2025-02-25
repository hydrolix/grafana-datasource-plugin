import { ErrorMessageBeautifier } from './errorBeautifier';

describe('ErrorMessageBeautifier', () => {
    let beautifier: ErrorMessageBeautifier;

    beforeEach(() => {
        beautifier = new ErrorMessageBeautifier();
    });

    test('should return null if input string does not contain valid JSON', () => {
        const input = "Invalid string without JSON";
        const result = beautifier.beautify(input);
        expect(result).toBeUndefined();
    });

    test('should return null if parsed JSON does not contain an error property', () => {
        const input = 'prefix {"query": "SELECT *"} suffix';
        const result = beautifier.beautify(input);
        expect(result).toBeUndefined();
    });

    test('should extract default CH message', () => {
        const input = 'prefix {"error": "Code: 123. DB::Exception: Something went wrong. (ABC)", "query": "SELECT * FROM table"} suffix';
        const result = beautifier.beautify(input);
        expect(result).toEqual("Something went wrong");
    });

    test('should extract default CH message multiline', () => {
        const input = 'error querying the database: clickhouse [execute]:: 400 code: { "error": "Code: 62. DB::Exception: Syntax error: failed at position 238 (\'GROUP8\') (line 1, col 238): GROUP8 BY time ORDER BY time\\nSETTINGS hdx_query_max_execution_time=60, hdx_query_admin_comment=\'db=${__dashboard}; dp=Throughput; du=${__user.login}\'\\n. Expected one of: token, OpeningRoundBracket, FILTER, RESPECT NULLS, IGNORE NULLS, OVER, OR, AND, IS NOT DISTINCT FROM, IS NULL, IS NOT NULL, BETWEEN, NOT BETWEEN, LIKE, ILIKE, NOT LIKE, NOT ILIKE, REGEXP, IN, NOT IN, GLOBAL IN, GLOBAL NOT IN, MOD, DIV, alias, AS, GROUP BY, WITH, HAVING, WINDOW, ORDER BY, LIMIT, OFFSET, FETCH, SETTINGS, UNION, EXCEPT, INTERSECT, INTO OUTFILE, FORMAT, end of query. (SYNTAX_ERROR)", "query": "SELECT toStartOfInterval(toDateTime(reqTimeSec), INTERVAL 2 second) as time, sum(totalBytes) / sum(transferTimeMSec/1000) as throughput FROM akamai.logs WHERE reqTimeSec >= toDateTime(1740403350) AND reqTimeSec <= toDateTime(1740406950) GROUP8 BY time ORDER BY time\\nSETTINGS hdx_query_max_execution_time=60, hdx_query_admin_comment=\'db=${__dashboard}; dp=Throughput; du=${__user.login}\'\\n" }'
        const result = beautifier.beautify(input);
        expect(result).toEqual("Syntax error: failed at position 238 ('GROUP8') (line 1, col 238): GROUP8 BY time ORDER BY time SETTINGS hdx_query_max_execution_time=60, hdx_query_admin_comment='db=${__dashboard}; dp=Throughput; du=${__user.login}' . Expected one of: token, OpeningRoundBracket, FILTER, RESPECT NULLS, IGNORE NULLS, OVER, OR, AND, IS NOT DISTINCT FROM, IS NULL, IS NOT NULL, BETWEEN, NOT BETWEEN, LIKE, ILIKE, NOT LIKE, NOT ILIKE, REGEXP, IN, NOT IN, GLOBAL IN, GLOBAL NOT IN, MOD, DIV, alias, AS, GROUP BY, WITH, HAVING, WINDOW, ORDER BY, LIMIT, OFFSET, FETCH, SETTINGS, UNION, EXCEPT, INTERSECT, INTO OUTFILE, FORMAT, end of query");
    });

    test('should override message for code 159 with timeout', () => {
        const input = 'prefix {"error": "Code: 159. DB::Exception: Timeout exceeded: elapsed 2.000005926 seconds, maximum: 2: While executing HdxSource.: While executing HdxPeerSource. (TIMEOUT_EXCEEDED)", "query": "SELECT 1"} suffix';
        const result = beautifier.beautify(input);
        expect(result).toEqual("Query exceeded time limit of 2 seconds");
    });

    test('should not override message for code 159 if timeout missed', () => {
        const input = 'prefix {"error": "Code: 159. DB::Exception: Some error. (XYZ)", "query": "SELECT 1"} suffix';
        const result = beautifier.beautify(input);
        expect(result).toEqual("Some error");
    });

    test('should extract Hydrolix message', () => {
        const input = 'prefix {"error": "<Error Hydrolix specific error message (Hydrolix v2)>", "query": "SELECT *"} suffix';
        const result = beautifier.beautify(input);
        expect(result).toEqual("Hydrolix specific error message");
    });

    test('should format singular unit when timeout equals 1', () => {
        const input = 'prefix {"error": "Code: 159. DB::Exception: Some error. (XYZ) elapsed 1 seconds, maximum: 1", "query": "SELECT 1"} suffix';
        const result = beautifier.beautify(input);
        expect(result).toEqual("Query exceeded time limit of 1 second");
    });
});
