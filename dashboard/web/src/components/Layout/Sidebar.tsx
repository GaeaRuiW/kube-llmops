import React from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Layout, Menu } from 'antd';
import type { MenuProps } from 'antd';
import {
  DashboardOutlined,
  CloudServerOutlined,
  ExperimentOutlined,
  BookOutlined,
  AppstoreOutlined,
  LineChartOutlined,
  CodeOutlined,
  FileTextOutlined,
  SettingOutlined,
  TeamOutlined,
  SafetyOutlined,
  KeyOutlined,
} from '@ant-design/icons';
import { useAuthStore } from '../../store/auth';
import { usePermission } from '../../hooks/usePermission';

const { Sider } = Layout;

type MenuItem = Required<MenuProps>['items'][number];

export const Sidebar: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const collapsed = useAuthStore((s) => s.sidebarCollapsed);
  const { hasPermission, hasAnyPermission } = usePermission();

  const items: MenuItem[] = [
    { key: '/overview', icon: <DashboardOutlined />, label: 'Overview' },

    { type: 'group', label: collapsed ? '' : 'Workloads', children: [
      hasPermission('models', 'view') ? { key: '/models', icon: <CloudServerOutlined />, label: 'Models' } : null,
      hasPermission('finetune', 'view') ? { key: '/finetune', icon: <ExperimentOutlined />, label: 'Fine-tuning' } : null,
      hasPermission('rag', 'view') ? { key: '/rag', icon: <BookOutlined />, label: 'RAG' } : null,
    ].filter(Boolean) as MenuItem[] },

    { type: 'group', label: collapsed ? '' : 'Services', children: [
      { key: '/services', icon: <AppstoreOutlined />, label: 'Services' },
    ]},

    { type: 'group', label: collapsed ? '' : 'Observe', children: [
      hasPermission('monitoring', 'view') ? { key: '/monitoring', icon: <LineChartOutlined />, label: 'Monitoring' } : null,
      { key: '/notebooks', icon: <CodeOutlined />, label: 'Notebooks' },
      { key: '/logs', icon: <FileTextOutlined />, label: 'Logs' },
    ].filter(Boolean) as MenuItem[] },

    ...(hasAnyPermission([['platform', 'view'], ['users', 'view']]) ? [
      { type: 'group' as const, label: collapsed ? '' : 'Admin', children: [
        hasPermission('platform', 'view') ? { key: '/platform', icon: <SettingOutlined />, label: 'Platform' } : null,
        hasPermission('users', 'view') ? { key: '/users', icon: <TeamOutlined />, label: 'Users' } : null,
        hasPermission('roles', 'view') ? { key: '/users/roles', icon: <SafetyOutlined />, label: 'Roles' } : null,
        hasPermission('permissions', 'view') ? { key: '/users/permissions', icon: <KeyOutlined />, label: 'Permissions' } : null,
      ].filter(Boolean) as MenuItem[] },
    ] : []),
  ];

  const selectedKey = '/' + location.pathname.split('/').filter(Boolean).slice(0, 1).join('/');

  return (
    <Sider
      collapsible
      collapsed={collapsed}
      trigger={null}
      width={220}
      collapsedWidth={64}
      style={{ height: '100vh', position: 'fixed', left: 0, top: 0 }}
    >
      <div style={{ height: 48, display: 'flex', alignItems: 'center', justifyContent: 'center', margin: '8px 0' }}>
        <span style={{ fontSize: collapsed ? 16 : 18, fontWeight: 700, color: '#1677ff' }}>
          {collapsed ? 'K' : 'kube-llmops'}
        </span>
      </div>
      <Menu
        mode="inline"
        selectedKeys={[selectedKey]}
        items={items}
        onClick={({ key }) => navigate(key)}
        style={{ borderRight: 0 }}
      />
    </Sider>
  );
};
