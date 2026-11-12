import { locationService, TemplateSrv } from "@grafana/runtime";
import { TypedVariableModel } from "@grafana/data";
import { ErrorExposer } from "./errorExposer";
import { ErrorMessageBeautifier } from "./errorBeautifier";
import { resolveExposeErrors } from "../defaultConfigs";

jest.mock("@grafana/runtime", () => ({
  ...jest.requireActual("@grafana/runtime"),
  locationService: { partial: jest.fn() },
}));

function templateSrvWith(variables: Array<Partial<TypedVariableModel>>) {
  return { getVariables: () => variables } as unknown as TemplateSrv;
}

const errorsVariable = {
  name: "hdx_query_errors",
  current: { value: "[]", text: "" },
} as unknown as TypedVariableModel;

describe("ErrorExposer", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("does not touch the URL when the dashboard has no matching variable", async () => {
    const exposer = new ErrorExposer(
      new ErrorMessageBeautifier(),
      templateSrvWith([{ name: "other" }]),
      resolveExposeErrors(undefined)
    );
    await exposer.addErrorToVariable("boom");
    expect(locationService.partial).not.toHaveBeenCalled();
  });

  it("writes the error to the matching variable", async () => {
    const exposer = new ErrorExposer(
      new ErrorMessageBeautifier(),
      templateSrvWith([errorsVariable]),
      resolveExposeErrors(undefined)
    );
    await exposer.addErrorToVariable("boom");
    expect(locationService.partial).toHaveBeenCalledTimes(1);
    const [params, replace] = (locationService.partial as jest.Mock).mock.calls[0];
    expect(replace).toBe(true);
    const written = JSON.parse(params["var-hdx_query_errors"]);
    expect(written).toHaveLength(1);
    expect(written[0].message).toContain("boom");
  });

  it("writes nothing when disabled", async () => {
    const exposer = new ErrorExposer(
      new ErrorMessageBeautifier(),
      templateSrvWith([errorsVariable]),
      resolveExposeErrors({ enabled: false })
    );
    await exposer.addErrorToVariable("boom");
    expect(locationService.partial).not.toHaveBeenCalled();
  });
});
