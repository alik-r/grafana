import { css } from '@emotion/css';

import { GrafanaTheme2 } from '@grafana/data';
import { t } from '@grafana/i18n';
import { sceneGraph } from '@grafana/scenes';
import { Dropdown, Menu, ToolbarButton, useStyles2 } from '@grafana/ui';

import { DashboardScene } from './DashboardScene';
import { VariableValueSelectWrapper } from './VariableControls';

export function DropdownVariableControls({ dashboard }: { dashboard: DashboardScene }) {
  const styles = useStyles2(getStyles);
  const variables = sceneGraph
    .getVariables(dashboard)!
    .useState()
    .variables.filter((v) => v.state.displayInDropdownMenu === true);

  if (variables.length === 0) {
    return null;
  }

  return (
    <Dropdown
      overlay={
        <Menu>
          {variables.map((variable) => (
            <div className={styles.menuItem} key={variable.state.key}>
              <VariableValueSelectWrapper variable={variable} />
            </div>
          ))}
        </Menu>
      }
    >
      <ToolbarButton
        aria-label={t('dashboard.controls.menu.aria-label', 'Dashboard controls menu')}
        title={t('dashboard.controls.menu.title', 'Dashboard controls')}
        icon="ellipsis-v"
        iconSize="md"
        narrow
      />
    </Dropdown>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  menuItem: css({
    padding: theme.spacing(0.5),
  }),
});
