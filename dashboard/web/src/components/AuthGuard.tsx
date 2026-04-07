import React, { useEffect, useState } from 'react';
import { Outlet, Navigate } from 'react-router-dom';
import { Spin } from 'antd';
import { useAuthStore } from '../store/auth';
import apiClient from '../api/client';

export const AuthGuard: React.FC = () => {
  const { token } = useAuthStore();
  const [ssoEnabled, setSsoEnabled] = useState<boolean | null>(null);

  useEffect(() => {
    apiClient.get('/auth/config')
      .then((res) => setSsoEnabled(res.data.sso_enabled))
      .catch(() => setSsoEnabled(false));
  }, []);

  // Still loading auth config
  if (ssoEnabled === null) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <Spin size="large" />
      </div>
    );
  }

  // SSO enabled but no token → redirect to login
  if (ssoEnabled && !token) {
    return <Navigate to="/login" replace />;
  }

  // SSO disabled or has token → render app
  return <Outlet />;
};
