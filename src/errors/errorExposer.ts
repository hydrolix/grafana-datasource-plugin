import { VariableWithOptions } from "@grafana/data/dist/types/types/templateVars";
import { locationService, TemplateSrv } from "@grafana/runtime";
import { ErrorMessageBeautifier } from "./errorBeautifier";
import { ErrorFixSuggestion, ExposeErrorsOptions } from "../types";
import { SOLUTION_TEMPLATES } from "./solutionTemplates";

export class ErrorExposer {
  constructor(
    private beautifier: ErrorMessageBeautifier,
    private templateSrv: TemplateSrv,
    private settings: ExposeErrorsOptions
  ) {}

  public async addErrorToVariable(message: string) {
    if (
      !this.settings ||
      !this.settings.enabled ||
      !this.settings.variableName ||
      !this.settings.maxCount ||
      !this.settings.ttl
    ) {
      return;
    }
    const json = this.beautifier.parseJson(message);
    if (json) {
      message = json.error;
    }

    const beautifiedMessage = this.beautifier.beautify(message);

    const variable = this.templateSrv
      .getVariables()
      .find(
        (v) => v.name === this.settings.variableName
      ) as VariableWithOptions;
    let errors: ErrorFixSuggestion[] = [];
    if (variable) {
      try {
        errors = JSON.parse(variable?.current?.value.toString());
        if (!errors.length) {
          errors = [];
        }
      } catch (e) {
        console.error(e);
      }
    }
    let error: ErrorFixSuggestion = {
      time: new Date().toISOString(),
      message: beautifiedMessage || message,
    };

    const suggestion = SOLUTION_TEMPLATES.find((s) =>
      new RegExp(s.regexp).test(message)
    );
    if (suggestion) {
      const match = new RegExp(suggestion.regexp).exec(message);
      error.template = suggestion.name;
      if (match?.groups) {
        error.groups = match.groups;
      } else {
        error.groups = {};
      }
    }
    errors.push(error);

    errors
      .sort((a: { time: string }, b: { time: string }) =>
        (a.time || "").localeCompare(b.time || "")
      )
      .reverse();

    errors = errors.filter(
      (e: { time: string }) =>
        new Date().getTime() - new Date(e.time).getTime() <
        this.settings.ttl! * 1000
    );
    errors = errors.slice(0, this.settings.maxCount);
    this.setVariable(errors);
  }

  private setVariable(errors: ErrorFixSuggestion[]) {
    const queryParam: { [name: string]: string } = {};
    queryParam[`var-${this.settings.variableName}`] = JSON.stringify(errors);
    locationService.partial(queryParam, true);
  }
}
