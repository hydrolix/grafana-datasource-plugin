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
  const editor = m.editor as any;
  if (editor._standaloneKeybindingService) {
    // grafana 11.x
    applyBinding(editor._standaloneKeybindingService);
  } else {
    // grafana 10.x
    editor
      .getEditors()
      .map((e: any) => e._standaloneKeybindingService)
      .filter((s: any) => s)
      .map(applyBinding);
  }
};

export const updateOptions = (m: Monaco) => {
  let options = {
    scrollBeyondLastLine: false,
  };
  const editor = m.editor as any;
  if (editor.updateOptions) {
    // grafana 11.x
    editor.updateOptions(options);
  } else {
    // grafana 10.x
    m.editor
      .getEditors()
      .filter((e) => e.updateOptions)
      .map((e) => e.updateOptions(options));
  }
};

export const getDefaultValue = (value: any, type: string) => {
  if (type === "boolean") {
    return value ? "1" : "0";
  } else if (value !== undefined) {
    return `${value}`;
  } else {
    return "";
  }
};
