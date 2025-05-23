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

export const underline = (
  m: Monaco | null,
  line: number,
  start: number,
  end: number
): void => {
  if (!m) {
    return;
  }
  let decorations = [
    {
      range: new m.Range(line, start, line, end),
      options: { inlineClassName: "myInlineDecoration" },
    },
  ];

  // @ts-ignore
  if (m.editor.createDecorationsCollection) {
    // grafana 11.x
    // @ts-ignore
    m.editor.createDecorationsCollection(decorations);
  } else {
    // grafana 10.x
    m.editor
      .getEditors()
      .filter((e) => e.createDecorationsCollection)
      .map((e) => e.createDecorationsCollection(decorations));
  }
};

export const removeUnderline = (m: Monaco | null): void => {
  if (!m) {
    return;
  }
  //const range = new m.Range(1, 0, Number.MAX_VALUE, Number.MAX_VALUE);
  // @ts-ignore
  if (m.editor.getDecorationsInRange) {
    // grafana 11.x
    // @ts-ignore
    const range = m.editor.getModel().getFullModelRange();
    // @ts-ignore
    m.editor.removeDecorations(
      // @ts-ignore
      m.editor
        // @ts-ignore
        .getDecorationsInRange(range)
        // @ts-ignore
        ?.filter((d) => d.options.inlineClassName === "myInlineDecoration")
        // @ts-ignore
        ?.map((d) => d.id) ?? []
    );
  } else {
    // grafana 10.x
    m.editor
      .getEditors()
      .filter((e) => e.getDecorationsInRange)
      .map((e) => {
        // @ts-ignore
        const range = e.getModel().getFullModelRange();
        return e.removeDecorations(
          e
            .getDecorationsInRange(range)
            ?.filter((d) => d.options.inlineClassName === "myInlineDecoration")
            .map((d) => d.id) ?? []
        );
      });
  }
};
