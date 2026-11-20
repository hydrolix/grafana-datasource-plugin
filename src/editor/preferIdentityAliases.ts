import { AdHocFilterKeys } from "../types";

/**
 * Opt-in presentation policy: hide every physical column that some
 * same-type identity alias stands for (`aliasFor`), so the alias is the one
 * offered. Order and every other key are preserved; several aliases of one
 * base all remain. Pure over the already-fetched key list, so Map columns,
 * array columns and quoted table names need no handling here.
 */
export function preferIdentityAliases(keys: AdHocFilterKeys[]): AdHocFilterKeys[] {
  const hiddenBases = new Set(
    keys
      .map((key) => key.aliasFor)
      .filter((base): base is string => !!base)
  );
  return hiddenBases.size
    ? keys.filter((key) => !hiddenBases.has(String(key.value ?? key.text)))
    : keys;
}
