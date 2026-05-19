import { Drawer, type DrawerProps } from 'antd';

const DEFAULT_DRAWER_WIDTH = '50%';

/** Project-wide drawer; default width 50% (see also global.less). */
export default function AppDrawer({ width = DEFAULT_DRAWER_WIDTH, ...rest }: DrawerProps) {
  return <Drawer width={width} {...rest} />;
}

export { DEFAULT_DRAWER_WIDTH };
