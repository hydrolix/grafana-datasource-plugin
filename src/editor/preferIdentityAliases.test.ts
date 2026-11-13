import { AdHocFilterKeys } from '../types';
import { preferIdentityAliases } from './preferIdentityAliases';

const key = (name: string, type = 'String', aliasFor?: string): AdHocFilterKeys => ({
  text: name,
  value: name,
  type,
  ...(aliasFor ? { aliasFor } : {}),
});

describe('preferIdentityAliases', () => {
  it('hides the base behind its identity alias, keeping unrelated and Map columns in order', () => {
    const keys = [
      key('cliIP'),
      key('unrelatedPhysical'),
      key('client_ip', 'String', 'cliIP'),
      key('attrs', 'Map(String, String)'),
    ];
    expect(preferIdentityAliases(keys)).toEqual([keys[1], keys[2], keys[3]]);
  });

  it('keeps every alias of one base', () => {
    const keys = [key('cliIP'), key('client_ip', 'String', 'cliIP'), key('src_ip', 'String', 'cliIP')];
    expect(preferIdentityAliases(keys).map((k) => k.value)).toEqual(['client_ip', 'src_ip']);
  });

  it('returns the input unchanged when no key carries aliasFor', () => {
    const keys = [key('cliIP'), key('computed'), key('tags', 'Array(String)')];
    expect(preferIdentityAliases(keys)).toBe(keys);
  });
});
