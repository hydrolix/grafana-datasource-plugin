import React, { useCallback, useMemo, useState } from "react";
import labels from "../labels";
import { AccessoryButton, InputGroup } from "@grafana/plugin-ui";
import {
  Box,
  Collapse,
  Combobox,
  ComboboxOption,
  Input,
  Stack,
  useStyles2,
} from "@grafana/ui";
import { useToggle } from "react-use";
import { GrafanaTheme2 } from "@grafana/data";
import { css } from "@emotion/css";
import { QuerySetting } from "../types";

interface Props {
  onSettingsChange: (settings: QuerySetting[]) => void;
  settings: QuerySetting[];
}

export function QuerySettings({ settings, onSettingsChange }: Props) {
  const styles = useStyles2(getStyles);
  const [isOpen, toggleOpen] = useToggle(false);
  const settingTypes = useMemo(
    () =>
      labels.components.querySettings.values.reduce((acc, current) => {
        acc[current.setting] = current.type;
        return acc;
      }, {} as { [setting: string]: string }),
    []
  );
  const [settingsArray, setSettingsArray] = useState(
    settings.map((setting) => ({
      ...setting,
      type: settingTypes[setting.setting],
    }))
  );
  const options = useMemo(() => {
    return labels.components.querySettings.values
      .map((v) => ({
        label: v.setting,
        value: v.setting,
        description: v.description,
        type: v.type,
      }))
      .filter((v) => !settingsArray.some((s) => s.setting === v.label));
  }, [settingsArray]);

  const showAdd = useMemo(() => {
    return !settingsArray.some((s) => !s.setting);
  }, [settingsArray]);

  const updateSettings = useCallback(
    (settings: Array<{ setting: string; value: string; type: string }>) => {
      setSettingsArray(settings);
      onSettingsChange(settings);
    },
    [onSettingsChange]
  );
  const newSetting = useCallback(() => {
    updateSettings([
      ...settingsArray,
      { setting: "", value: "", type: "string" },
    ]);
  }, [settingsArray, updateSettings]);

  const onNameChange = useCallback(
    (index: number, setting: ComboboxOption) => {
      let copy = [...settingsArray];
      copy[index] = {
        setting: setting.value,
        value: "",
        type: settingTypes[setting.value],
      };
      updateSettings(copy);
    },
    [settingTypes, settingsArray, updateSettings]
  );
  const onValueUpdate = useCallback(
    (key: string, value: string) => {
      updateSettings(
        settingsArray.map((setting) => {
          if (setting.setting === key) {
            return {
              setting: setting.setting,
              value: value,
              type: setting.type,
            };
          } else {
            return setting;
          }
        })
      );
    },
    [settingsArray, updateSettings]
  );
  const onDeleteSetting = useCallback(
    (setting: string) => {
      updateSettings(settingsArray.filter((v) => v.setting !== setting));
    },
    [settingsArray, updateSettings]
  );

  const settingInput = (setting: {
    setting: string;
    value: string;
    type: string;
  }) => {
    if (setting.type === "boolean") {
      return (
        <Combobox
          width={10}
          options={[
            { value: "1", label: "Yes" },
            { value: "0", label: "No" },
          ]}
          onChange={(v) => onValueUpdate(setting.setting, v.value)}
          value={setting.value ?? "0"}
        ></Combobox>
      );
    } else if (setting.type === "textarea") {
      return (
        <Input
          name={setting.setting}
          width={80}
          value={setting.value ?? ""}
          onChange={(e) =>
            onValueUpdate(setting.setting, e.currentTarget.value)
          }
          aria-label={setting.setting}
        />
      );
    } else if (setting.type === "number") {
      return (
        <Input
          name={setting.setting}
          width={20}
          value={setting.value ?? ""}
          onChange={(e) =>
            onValueUpdate(setting.setting, e.currentTarget.value)
          }
          aria-label={setting.setting}
        />
      );
    } else {
      return (
        <Input
          name={setting.setting}
          width={40}
          value={setting.value ?? ""}
          onChange={(e) =>
            onValueUpdate(setting.setting, e.currentTarget.value)
          }
          aria-label={setting.setting}
        />
      );
    }
  };

  return (
    <>
      <Box backgroundColor={"secondary"}>
        <div className={styles.wrapper}>
          <Collapse
            collapsible
            isOpen={isOpen}
            onToggle={toggleOpen}
            label={
              <Stack gap={0}>
                <h6 className={styles.title}>
                  {labels.components.querySettings.label}
                </h6>
                {!isOpen && (
                  <div className={styles.description}>
                    {settings.map((x, i) => (
                      <span key={i}>
                        {x.setting}={x.value}
                      </span>
                    ))}
                  </div>
                )}
              </Stack>
            }
            className={styles.collapse}
          >
            <div className={styles.body}>
              {settingsArray.map((setting, i) => (
                <InputGroup key={i}>
                  <Combobox
                    options={options}
                    onChange={(e) => onNameChange(i, e)}
                    width={"auto"}
                    value={setting.setting}
                    minWidth={10}
                    maxWidth={40}
                  ></Combobox>
                  <Input disabled={true} value={"="} width={3}></Input>
                  {settingInput(setting)}
                  <AccessoryButton
                    aria-label={"delete setting"}
                    icon="times"
                    variant="secondary"
                    onClick={() => onDeleteSetting(setting.setting)}
                  />
                </InputGroup>
              ))}
              {showAdd && (
                <AccessoryButton
                  aria-label={"new setting"}
                  icon="plus"
                  variant="secondary"
                  onClick={newSetting}
                />
              )}
            </div>
          </Collapse>
        </div>
      </Box>
    </>
  );
}

const getStyles = (theme: GrafanaTheme2) => {
  return {
    collapse: css({
      backgroundColor: "unset",
      border: "unset",
      marginBottom: 0,
    }),
    wrapper: css({
      width: "100%",
      display: "flex",
      justifyContent: "space-between",
      alignItems: "baseline",
    }),
    title: css({
      flexShrink: 0,
      fontSize: theme.typography.bodySmall.fontSize,
      fontWeight: theme.typography.fontWeightMedium,
      margin: 0,
    }),
    description: css({
      color: theme.colors.text.secondary,
      fontSize: theme.typography.bodySmall.fontSize,
      fontWeight: theme.typography.bodySmall.fontWeight,
      paddingLeft: theme.spacing(2),
      gap: theme.spacing(2),
      display: "flex",
      flexWrap: "wrap",
    }),
    body: css({
      display: "flex",
      gap: theme.spacing(2),
      flexWrap: "wrap",
    }),
  };
};
