import React from 'react';
import { Layout } from 'antd';
import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { Header } from './Header';
import { useAuthStore } from '../../store/auth';

const { Content } = Layout;

export const AppLayout: React.FC = () => {
  const collapsed = useAuthStore((s) => s.sidebarCollapsed);

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sidebar />
      <Layout style={{ marginLeft: collapsed ? 64 : 220, transition: 'margin-left 0.2s' }}>
        <Header />
        <Content style={{ margin: 16, padding: 24, background: 'var(--ant-color-bg-container, #fff)', borderRadius: 8, minHeight: 360 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
};
