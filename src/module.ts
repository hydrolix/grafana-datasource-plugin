import { DataSourcePlugin } from "@grafana/data";
import { DataSource } from "./datasource";
import { ConfigEditor } from "./components/ConfigEditor";
import { QueryEditor } from "./components/QueryEditor";
import { HdxQuery, HdxDataSourceOptions } from "./types";

export const plugin = new DataSourcePlugin<
  DataSource,
  HdxQuery,
  HdxDataSourceOptions
>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);
