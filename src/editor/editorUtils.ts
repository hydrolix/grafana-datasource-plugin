import { Monaco } from "@grafana/ui";
import { Props } from "../components/QueryEditor";

export const applyHotKey = (m: Monaco, props: Props) => {
  let applyBinding = (service: any) => {
    service.addDynamicKeybinding(
      "executeQuery",
      m.KeyMod.CtrlCmd | m.KeyCode.Enter,
      props.onRunQuery
    );
    service.addDynamicKeybinding(
      "executeQuery",
      m.KeyMod.WinCtrl | m.KeyCode.Enter,
      props.onRunQuery
    );
  };
  // @ts-ignore
  if (m.editor._standaloneKeybindingService) {
    // grafana 11.x
    // @ts-ignore
    applyBinding(m.editor._standaloneKeybindingService);
  } else {
    // grafana 10.x
    m.editor
      .getEditors()
      // @ts-ignore
      .map((e) => e._standaloneKeybindingService)
      .filter((s) => s)
      .map(applyBinding);
  }
};

export const updateOptions = (m: Monaco) => {
  let options = {
    scrollBeyondLastLine: false,
  };
  // @ts-ignore
  if (m.editor.updateOptions) {
    // grafana 11.x
    // @ts-ignore
    m.editor.updateOptions(options);
  } else {
    // grafana 10.x
    m.editor
      .getEditors()
      .filter((e) => e.updateOptions)
      .map((e) => e.updateOptions(options));
  }
};
