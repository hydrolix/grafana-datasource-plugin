import {
  BaseVariableModel,
  CustomVariableModel,
  LoadingState,
  VariableHide,
  VariableOption,
} from "@grafana/data";

export const initialVariableModelState: BaseVariableModel = {
  id: "00000000-0000-0000-0000-000000000000",
  rootStateKey: null,
  name: "",
  type: "query",
  global: false,
  index: -1,
  hide: VariableHide.dontHide,
  skipUrlSync: false,
  state: LoadingState.NotStarted,
  error: null,
  description: null,
};

export const initialCustomVariableModelState: CustomVariableModel = {
  ...initialVariableModelState,
  type: "custom",
  multi: false,
  includeAll: false,
  allValue: null,
  query: "",
  options: [],
  current: {} as VariableOption,
};

export const fooVariable: CustomVariableModel = {
  ...initialCustomVariableModelState,
  id: "foo",
  name: "foo",
  current: {
    value: "templatedFoo",
    text: "templatedFoo",
    selected: true,
  },
  options: [{ value: "templatedFoo", text: "templatedFooo", selected: true }],
  multi: false,
};

export const adHocTableVariable: CustomVariableModel = {
  ...initialCustomVariableModelState,
  id: "table",
  name: "table",
  current: {
    value: "table",
    text: "table",
    selected: true,
  },
  options: [{ value: "table", text: "table", selected: true }],
  multi: false,
};

export const adHocTimeColumnVariable: CustomVariableModel = {
  ...initialCustomVariableModelState,
  id: "timefilter",
  name: "timefilter",
  current: {
    value: "timefilter",
    text: "timefilter",
    selected: true,
  },
  options: [{ value: "timefilter", text: "timefilter", selected: true }],
  multi: false,
};
