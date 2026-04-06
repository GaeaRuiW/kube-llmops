import React from 'react';
import { Layout, Space, Dropdown, Avatar, Typography, Select } from 'antd';
import type { MenuProps } from 'antd';
import { UserOutlined, LogoutOutlined, ProfileOutlined, MenuFoldOutlined, MenuUnfoldOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../../store/auth';
import { ThemeToggle } from '../ThemeToggle';

const { Header: AntHeader } = Layout;
const { Text } = Typography;

export const Header: React.FC = () => {
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const collapsed = useAuthStore((s) => s.sidebarCollapsed);
  const toggleSidebar = useAuthStore((s) => s.toggleSidebar);
  const namespace = useAuthStore((s) => s.namespace);
  const setNamespace = useAuthStore((s) => s.setNamespace);
  const logout = useAuthStore((s) => s.logout);

  const userMenuItems: MenuProps['items'] = [
    { key: 'profile', icon: <ProfileOutlined />, label: 'Profile', onClick: () => navigate('/profile') },
    { type: 'divider' },
    { key: 'logout', icon: <LogoutOutlined />, label: 'Logout', onClick: () => { logout(); navigate('/login'); } },
  ];

  return (
    <AntHeader style={{ padding: '0 24px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', background: 'transparent' }}>
      <Space>
        {React.createElement(collapsed ? MenuUnfoldOutlined : MenuFoldOutlined, {
          onClick: toggleSidebar,
          style: { fontSize: 18, cursor: 'pointer' },
        })}
        <Select
          value={namespace}
          onChange={setNamespace}
          style={{ width: 160 }}
          size="small"
          options={[
            { value: 'default', label: 'default' },
            { value: 'kube-llmops', label: 'kube-llmops' },
          ]}
        />
      </Space>
      <Space size="middle">
        <ThemeToggle />
        <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
          <Space style={{ cursor: 'pointer' }}>
            <Avatar size="small" icon={<UserOutlined />} />
            <Text>{user?.displayName || 'User'}</Text>
          </Space>
        </Dropdown>
      </Space>
    </AntHeader>
  );
};
