import React from 'react';
import { Button, Tooltip } from 'antd';
import { SunOutlined, MoonOutlined, DesktopOutlined } from '@ant-design/icons';
import { useTheme } from '../hooks/useTheme';

export const ThemeToggle: React.FC = () => {
  const { themeMode, cycleTheme } = useTheme();

  const icon = themeMode === 'light' ? <SunOutlined /> :
               themeMode === 'dark' ? <MoonOutlined /> :
               <DesktopOutlined />;

  const label = themeMode === 'auto' ? 'System' : themeMode;

  return (
    <Tooltip title={`Theme: ${label}`}>
      <Button type="text" icon={icon} onClick={cycleTheme} />
    </Tooltip>
  );
};
