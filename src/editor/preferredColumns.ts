import { DataQueryResponse, Field } from '@grafana/data';

export interface PreferredColumn {
  name: string;
  type: string;
}

/** Read the complete schema, including names hidden from new picker choices. */
export function columnsFromResponse(response: DataQueryResponse) {
  if (response.errors?.length) {
    throw new Error(response.errors[0].message || 'Cannot read column metadata');
  }
  const fields: Field[] = response.data[0]?.fields || [];
  const values = (name: string): any[] => fields.find((field) => field.name === name)?.values || [];
  const names: string[] = values('name');
  const types = values('type');
  const defaults = values('default_type');
  const expressions = values('default_expression');
  return names.map((name, i) => ({
    name,
    type: types[i] as string,
    isAlias: defaults[i] === 'ALIAS',
    expression: String(expressions[i] || '').trim(),
  }));
}

/**
 * Experimental presentation policy. Prefer a same-type direct SQL identity
 * alias over its physical base. Physical lookalikes,
 * computed aliases, missing targets and cycles must remain visible.
 */
export function preferredColumnsFromResponse(response: DataQueryResponse): PreferredColumn[] {
  const columns = columnsFromResponse(response);
  const byName = new Map(columns.map((column) => [column.name, column]));
  // Prefer a proven identity alias over its physical base, without renaming
  // either SQL binding. Computed/type-changing/unresolved aliases stay as-is.
  const hiddenBases = new Set<string>();
  for (const column of columns) {
    if (!column.isAlias) {
      continue;
    }
    const match = /^(?:`([A-Za-z_][A-Za-z0-9_]*)`|([A-Za-z_][A-Za-z0-9_]*))$/.exec(column.expression);
    const base = match && byName.get(match[1] || match[2]);
    if (base && !base.isAlias && base.type === column.type) {
      hiddenBases.add(base.name);
    }
  }
  return columns.filter(({ name }) => !hiddenBases.has(name)).map(({ name, type }) => ({ name, type }));
}
