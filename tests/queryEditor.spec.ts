import { test, expect } from '@grafana/plugin-e2e';
import { selectors } from '@grafana/e2e-selectors'

test('smoke: should render query editor', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await expect(panelEditPage.getByGrafanaSelector(selectors.components.CodeEditor.container)).toBeVisible();
});
