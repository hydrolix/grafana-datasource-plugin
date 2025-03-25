import { AdHocVariableFilter } from "@grafana/data";
import { MetadataProvider } from "./editor/metadataProvider";

export const AD_HOC_MACRO = "$__adHocFilter()";

export class AdHocFilterApplier {
  constructor(
    private metadataProvider: MetadataProvider,
    private getTableFn: (sql: string) => string
  ) {}
  async apply(
    rawQuery: string,
    filters?: AdHocVariableFilter[]
  ): Promise<string> {
    if (!rawQuery) {
      return rawQuery;
    }

    let hasMacro = rawQuery.includes(AD_HOC_MACRO);
    if (hasMacro) {
      let condition;

      if (filters?.length) {
        let table = this.getTableFn(rawQuery);
        let keys = (await this.metadataProvider.tableKeys(table)).map(
          (k) => k.value
        );
        condition = filters
          .filter((f) => !f.key.includes(".") || f.key.includes(table))
          .filter((f) => keys.includes(keyToColumnAndTable(f.key)[0]))
          .map(this.getFilterExpression)
          .join(" AND ");
      }
      if (!condition) {
        condition = "1=1";
      }
      rawQuery = rawQuery.replaceAll(AD_HOC_MACRO, condition);
    }
    return rawQuery;
  }

  getFilterExpression(filter: AdHocVariableFilter): string {
    let [key, _] = keyToColumnAndTable(filter.key);
    if (filter.value === "null") {
      if (filter.operator === "=") {
        return `${key} IS NULL`;
      } else if (filter.operator === "!=") {
        return `${key} IS NOT NULL`;
      } else {
        throw new Error(
          `${key}: operator '${filter.operator}' can not be applied to NULL value`
        );
      }
    } else if (filter.operator === "=~") {
      return `toString(${key}) LIKE '${filter.value}'`;
    } else if (filter.operator === "!~") {
      return `toString(${key}) NOT LIKE '${filter.value}'`;
    } else {
      return `${key} ${filter.operator} '${filter.value}'`;
    }
  }
}

export const keyToColumnAndTable = (
  key: string
): [string, string | undefined] => {
  let dotIndex = key.lastIndexOf(".");
  if (dotIndex === -1) {
    return [key, undefined];
  } else {
    return [key.substring(dotIndex + 1), key.substring(0, dotIndex)];
  }
};
