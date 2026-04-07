import React, { useEffect } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { Spin, Typography } from 'antd';
import { useAuthStore } from '../../store/auth';
import apiClient from '../../api/client';

const { Text } = Typography;

const LoginCallback: React.FC = () => {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const { setToken, setUser, setPermissions } = useAuthStore();

  useEffect(() => {
    const idToken = params.get('id_token');
    if (!idToken) {
      navigate('/login', { replace: true });
      return;
    }
    // Store token first
    setToken(idToken);

    // Fetch user info with the new token
    apiClient.get('/auth/me', {
      headers: { Authorization: `Bearer ${idToken}` },
    }).then((res) => {
      if (res.data.user) {
        setUser(res.data.user);
        setPermissions(res.data.permissions ?? []);
      }
      navigate('/overview', { replace: true });
    }).catch(() => {
      // Token might be valid but /auth/me needs JWT middleware
      // Just navigate to overview — the token will be used for subsequent requests
      navigate('/overview', { replace: true });
    });
  }, [params, navigate, setToken, setUser, setPermissions]);

  return (
    <div style={{
      display: 'flex', justifyContent: 'center', alignItems: 'center',
      minHeight: '100vh', flexDirection: 'column', gap: 16,
    }}>
      <Spin size="large" />
      <Text type="secondary">Completing login...</Text>
    </div>
  );
};

export default LoginCallback;
