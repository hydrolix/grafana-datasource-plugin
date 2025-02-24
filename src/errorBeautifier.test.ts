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

    test('should extract CH message for non-159 error code', () => {
        const input = 'prefix {"error": "Code: 123. DB::Exception: Something went wrong. (ABC)", "query": "SELECT * FROM table"} suffix';
        const result = beautifier.beautify(input);
        expect(result).toEqual("Something went wrong");
    });

    test('should override message for code 159 when timeout exists', () => {
        const input = 'prefix {"error": "Code: 159. DB::Exception: Timeout exceeded: elapsed 2.000005926 seconds, maximum: 2: While executing HdxSource.: While executing HdxPeerSource. (TIMEOUT_EXCEEDED)", "query": "SELECT 1"} suffix';
        const result = beautifier.beautify(input);
        expect(result).toEqual("Query exceeded time limit of 2 seconds");
    });

    test('should not override message for code 159 if timeout is not found', () => {
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
