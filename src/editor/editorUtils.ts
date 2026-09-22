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
    // Monaco hangs the keybinding service off the editor namespace itself.
    applyBinding(editor._standaloneKeybindingService);
  } else {
    // Older shape: reach it through each live editor instance. This is a
    // capability probe, not a version check — `_standaloneKeybindingService`
    // is Monaco-private and has relocated before — so the arm is kept even
    // though no Grafana at the current `>=11.0.0` floor is known to need it.
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
    // Monaco applies options namespace-wide.
    editor.updateOptions(options);
  } else {
    // Older shape: apply per live editor instance. Capability probe, not a
    // version check — see `applyHotKey` above.
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
