import { AdHocVariableFilter, TypedVariableModel } from "@grafana/data";
import { MacrosApplier } from "./macrosApplier";

export class MacrosService {
  private macrosApplierList: MacrosApplier[] = [];

  async applyMacros(sql: string, context: Context): Promise<string> {
    return this.macrosApplierList.reduce(
      async (sqlPromise, applier) =>
        await applier.applyMacros(await sqlPromise, context),
      Promise.resolve(sql)
    );
  }

  registerMacros(macrosApplier: MacrosApplier) {
    this.macrosApplierList.push(macrosApplier);
  }
}

export interface Context {
  filters?: AdHocVariableFilter[];
  templateVars: TypedVariableModel[];
  replaceFn: (s: string) => string;
  intervalMs?: number;
}

export const emptyContext: Context = {
  filters: [],
  templateVars: [],
  replaceFn: (s) => s,
};
