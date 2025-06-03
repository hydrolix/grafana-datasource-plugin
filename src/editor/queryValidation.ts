import { ValidationResult } from "../types";
import { traverseTree } from "../ast";

const fromWithoutWhere = (ast: any): ValidationResult | null => {
  const errorNode = traverseTree(
    ast,
    (node) =>
      node.SelectItems &&
      !node.Where &&
      node.From &&
      !traverseTree(node.From, (n) => n.Select)
  );
  if (errorNode) {
    return {
      warning: "No WHERE clause for select query",
    };
  }
  return null;
};

const validators: Array<(ast: any) => ValidationResult | null> = [
  fromWithoutWhere,
];

export const validateQuery = (ast: any): ValidationResult => {
  for (const validator of validators) {
    const result = validator(ast);
    if (result) {
      return result;
    }
  }
  return {};
};
