class HdxError {
    code?: number;
    message: string
    query?: string

    constructor(code: number | undefined, message: string, query: string | undefined) {
        this.code = code;
        this.message = message;
        this.query = query;
    }
}

export class ErrorMessageBeautifier {
    private static readonly CH_CODE_REGEX = /^Code:\s+(\d+)\./
    private static readonly CH_MESSAGE_REGEX = /DB::Exception:\s+(?<message>.*?)(?:\.(?=\s+\(\w+\)$))?\s+\(\w+\)$/m
    private static readonly CH_TIMEOUT_REGEX = /elapsed\s+(?<elapsed>\d+(\.\d+)?)\s+seconds,\s+maximum:\s(?<timeout>\d+)/
    private static readonly HYDROLIX_MESSAGE_REGEX = /^<\w+\s+(?<message>.+)\s+\(Hydrolix.+\)>$/m

    public beautify(s: string): string | undefined {
        const error = this.parse(s);
        if (!error) {
            return undefined
        }

        let message: string | undefined = error.message.match(ErrorMessageBeautifier.CH_MESSAGE_REGEX)?.groups?.message
        if (error.code === 159) {
            let timeout = error.message.match(ErrorMessageBeautifier.CH_TIMEOUT_REGEX)?.groups?.timeout
            if (timeout) {
                message = `Query exceeded time limit of ${timeout} ${this.formatUnit(Number(timeout), "second", "seconds")}`
            }
        } else if (!error.code) {
            message = error.message.match(ErrorMessageBeautifier.HYDROLIX_MESSAGE_REGEX)?.groups?.message ?? error.message
        }

        return message
    }

    private parse(s: string): HdxError | undefined {
        const start = s.indexOf("{")
        const end = s.lastIndexOf("}")
        if (start < 0 || end < 0) {
            return undefined
        }

        const parsed = JSON.parse(s.substring(start, end + 1));
        if (!parsed || !(parsed?.error)) {
            return undefined
        }

        let code: number | undefined = Number(parsed.error.match(ErrorMessageBeautifier.CH_CODE_REGEX)?.[1])
        if (Number.isNaN(code)) {
            code = undefined
        }

        return new HdxError(code, parsed.error, parsed?.query)
    }

    private formatUnit(value: number, singular: string, plural: string): string {
        return value > 1 ? plural : singular
    }
}
