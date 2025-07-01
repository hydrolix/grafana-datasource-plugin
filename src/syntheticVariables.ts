export const replace = (
  s: string,
  vars: { [v: string]: () => string }
): string => {
  return Object.entries(vars).reduce((acc, [k, v]) => {
    return acc.replaceAll(`$\{__hydrolix.${k}}`, v());
  }, s);
  // skip removing not replaced variables with empty string
  //  .replaceAll(/\$\{(?:\w|\d|\.)*}|\$__\w*/g, "");
};
