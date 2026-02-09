class HdxDBError {
  code?: number;
  message: string;
  query?: string;

  constructor(
    code: number | undefined,
    message: string,
    query: string | undefined
  ) {
    this.code = code;
    this.message = message;
    this.query = query;
  }
}

export class ErrorMessageBeautifier {
  private static readonly AUTH_NATIVE_ERROR =
    /^.*\s\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}:\d{1,5}\s(failed\sto\sauthenticate\suser\s'.*').*TurbineApiAuthenticatorError.*$/;
  private static readonly AUTH_HTTP_ERROR =
    /^<TurbineApiAuthenticatorError\s(api login failed with provided username\/password\s\S*)/;
  private static readonly CONNECTION_HTTP_REFUSED =
    /^Post.* \d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}:\d{1,5}:\s(.*)$/;
  private static readonly CONNECTION_NATIVE_REFUSED =
    /^dial\stcp.* \d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}:\d{1,5}:\s(.*)$/;
  private static readonly CH_CODE_REGEX = /Code:\s+(\d+)[.|,]/i;
  private static readonly CH_MESSAGE_HTTP_REGEX =
    /DB::Exception:\s+(?<message>.*?)(?:\.(?=\s+\(\w+\)$))?\s+\(\w+\)/m;
  private static readonly CH_MESSAGE_NATIVE_REGEX =
    /code:\s(?<code>\d+),\smessage:(?<message>.+)$/;
  private static readonly CH_DATABASE_ERROR_REGEX =
    /<(?:ChTurbineDatabaseError|DatabaseError) (?<message>.+)>/;
  private static readonly HYDROLIX_MESSAGE_REGEX =
    /^<\w+\s+(?<message>.+)\s+\(Hydrolix.+\)>$/m;

  public beautify(s: string): string | undefined {
    let message: string | undefined;

    message = this.handleConnectionErrors(s);
    if (message) {
      return message;
    }

    const json = this.parseJson(s);

    message = this.handleAuthError(s, json);
    if (message) {
      return message;
    }

    message = this.handleDBErrors(s, json);
    if (message) {
      return message;
    }
    return undefined;
  }

  private handleDBErrors(s: string, json: any): string | undefined {
    const error = this.parseDBError(s, json);
    if (!error) {
      return undefined;
    }

    let message: string | undefined = error.message.match(
      ErrorMessageBeautifier.CH_MESSAGE_HTTP_REGEX
    )?.groups?.message;
    if (!message) {
      message = error.message.match(
        ErrorMessageBeautifier.CH_MESSAGE_NATIVE_REGEX
      )?.groups?.message;
    }
    if (!message) {
      message = error.message.match(
        ErrorMessageBeautifier.CH_DATABASE_ERROR_REGEX
      )?.groups?.message;
    }
    if (!error.code) {
      message =
        error.message.match(ErrorMessageBeautifier.HYDROLIX_MESSAGE_REGEX)
          ?.groups?.message ?? error.message;
    }

    return message;
  }

  private handleConnectionErrors(s: string): string | undefined {
    return [
      ErrorMessageBeautifier.CONNECTION_NATIVE_REFUSED,
      ErrorMessageBeautifier.CONNECTION_HTTP_REFUSED,
    ]
      .map((regex) => {
        if (regex.test(s)) {
          let match = regex.exec(s)!;
          if (match.length > 1) {
            return match[1];
          }
        }
        return undefined;
      })
      .find((e) => !!e);
  }

  private parseDBError(s: string, parsed: any) {
    const error = (parsed?.error || s).replace(/\r?\n/g, " ");
    let code: number | undefined = Number(
      error.match(ErrorMessageBeautifier.CH_CODE_REGEX)?.[1]
    );
    if (Number.isNaN(code)) {
      code = undefined;
    }
    return new HdxDBError(code, error, parsed?.query);
  }

  public parseJson(s: string): any {
    const start = s.indexOf("{");
    const end = s.lastIndexOf("}");
    if (start < 0 || end < 0) {
      return undefined;
    }

    try {
      return JSON.parse(s.substring(start, end + 1));
    } catch (ex) {
      return undefined;
    }
  }

  private handleAuthError(s: string, json: any): string | undefined {
    let message = undefined;
    try {
      message = this.handleNativeAuthError(s, json);
      if (message) {
        return message;
      }
      message = this.handleHTTPAuthError(json);
      if (message) {
        return message;
      }
    } catch (ex) {
      console.error(ex);
    }
    return message;
  }

  private handleHTTPAuthError(json: any): string | undefined {
    if (json && json["error"]) {
      const innerJson = this.parseJson(json["error"]);
      return this.parseAuthError(
        json["error"],
        innerJson,
        ErrorMessageBeautifier.AUTH_HTTP_ERROR
      );
    }
    return undefined;
  }

  private handleNativeAuthError(s: string, json: any): string | undefined {
    return this.parseAuthError(
      s,
      json,
      ErrorMessageBeautifier.AUTH_NATIVE_ERROR
    );
  }
  private parseAuthError(
    error: string,
    json: any,
    regex: RegExp
  ): string | undefined {
    if (regex.test(error)) {
      let message = "";
      let match = regex.exec(error)!;
      if (match.length > 1) {
        message += `${match[1]}:\n`;
      }
      if (json) {
        message += Object.keys(json)
          .map(
            (key) =>
              `${key}: ${
                Array.isArray(json[key]) ? json[key].join(", ") : json[key]
              }`
          )
          .join("\n");
      }
      return message;
    }
    return undefined;
  }
}
