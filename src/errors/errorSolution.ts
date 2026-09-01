import { SOLUTION_TEMPLATES } from "./solutionTemplates";

export interface SolutionMatch {
  name: string;
  groups: { [name: string]: string };
  // The template text with {placeholder} occurrences substituted from the
  // captured groups. Placeholders without a captured value are left verbatim
  // ({{...}} escapes in the templates stay literal the same way).
  solution: string;
}

/**
 * Classifies a raw error message against the curated solution templates.
 * Shared by the error-exposer variable (which stores the template name and
 * groups for the error panel to render) and the Assistant explain action
 * (which sends the rendered solution as grounding for the model).
 */
export const matchSolutionTemplate = (
  message: string
): SolutionMatch | undefined => {
  const template = SOLUTION_TEMPLATES.find((s) =>
    new RegExp(s.regexp).test(message)
  );
  if (!template) {
    return undefined;
  }
  const groups = new RegExp(template.regexp).exec(message)?.groups ?? {};
  return {
    name: template.name,
    groups,
    solution: renderSolutionTemplate(template.template, groups),
  };
};

export const renderSolutionTemplate = (
  template: string,
  groups: { [name: string]: string }
): string =>
  template.replace(/\{(\w+)}/g, (whole, key: string) => groups[key] ?? whole);
