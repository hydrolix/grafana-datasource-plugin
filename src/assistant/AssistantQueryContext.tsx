import { useRef } from "react";
import { useProvidePageContext } from "@grafana/assistant";
import { TimeRange } from "@grafana/data";
import { useDebounce } from "react-use";
import { DataSource } from "../datasource";
import {
  AssistantContextInput,
  buildAssistantContextItems,
  resolveTableRef,
} from "./context";

// The QueryEditor renders on panel edit, Explore, and dashboards; gating is
// by mount (see below), not by URL.
const EDITOR_URL_PATTERN = /.*/;

export interface AssistantQueryContextProps {
  datasource: DataSource;
  rawSql: string;
  range?: TimeRange;
}

/**
 * Publishes the current query state as Assistant page context. Table
 * resolution goes through the backend /ast parse and schema through the
 * metadata provider's memoized fetches — both debounced behind the same
 * 300ms the interpolation preview already uses, so the editor never waits
 * on them.
 *
 * Render this only while Assistant is available: useProvidePageContext
 * registers globally on mount, and mounting is the gate that keeps
 * Assistant-less installs at zero registrations. Unmounting unregisters.
 */
export function AssistantQueryContext({
  datasource,
  rawSql,
  range,
}: AssistantQueryContextProps) {
  const setContext = useProvidePageContext(EDITOR_URL_PATTERN);
  // Guards against out-of-order publishes when an older parse or schema
  // fetch resolves after a newer one.
  const generation = useRef(0);

  useDebounce(
    async () => {
      const current = ++generation.current;
      const jsonData = datasource.instanceSettings?.jsonData;
      const input: AssistantContextInput = {
        rawSql,
        datasourceUid: datasource.uid,
        datasourceHost: jsonData?.host,
        timeRange: range,
      };

      if (rawSql) {
        try {
          const table = resolveTableRef(await datasource.getAst(rawSql));
          if (table) {
            const database = table.database ?? jsonData?.defaultDatabase;
            input.table = { table: table.table, database };
            if (database) {
              const identifier = { schema: database, table: table.table };
              const [columns, primaryTimeColumn] = await Promise.all([
                datasource.metadataProvider.columns(identifier),
                datasource.metadataProvider.primaryKey(identifier),
              ]);
              input.columns = columns;
              input.primaryTimeColumn = primaryTimeColumn;
            }
          }
        } catch {
          // Mid-edit SQL rarely parses and schema fetches can fail; the
          // basic context (SQL, host, range) is still worth publishing.
        }
      }

      if (current === generation.current) {
        setContext(buildAssistantContextItems(input));
      }
    },
    300,
    [rawSql, range, datasource, setContext]
  );

  return null;
}
