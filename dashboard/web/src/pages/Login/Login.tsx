import React from 'react';
import { Button, Card, Typography, Space } from 'antd';
import { LoginOutlined } from '@ant-design/icons';

const { Title, Text } = Typography;

const Login: React.FC = () => {
  return (
    <div style={{
      display: 'flex', justifyContent: 'center', alignItems: 'center',
      minHeight: '100vh', background: '#f0f2f5',
    }}>
      <Card style={{ width: 400, textAlign: 'center' }}>
        <Space direction="vertical" size="large" style={{ width: '100%' }}>
          <Title level={2} style={{ margin: 0 }}>kube-llmops</Title>
          <Text type="secondary">Kubernetes-native LLMOps Platform</Text>
          <Button
            type="primary"
            size="large"
            icon={<LoginOutlined />}
            block
            onClick={() => { window.location.href = '/api/v1/auth/login'; }}
          >
            Login with Keycloak
          </Button>
        </Space>
      </Card>
    </div>
  );
};

export default Login;
