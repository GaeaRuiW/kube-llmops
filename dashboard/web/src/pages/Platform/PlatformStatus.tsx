import React from 'react';
import { Card, Descriptions, Table, Tag, Typography, Spin } from 'antd';
import { useQuery } from '@tanstack/react-query';
import apiClient from '../../api/client';

const { Title } = Typography;

interface Component {
  name: string;
  phase: string;
  endpoint: string;
  version?: string;
}

interface Platform {
  metadata: { name: string; namespace: string };
  spec: Record<string, unknown>;
  status: { phase: string; components: Record<string, { phase: string; endpoint: string }> };
}

const PlatformStatus: React.FC = () => {
  const { data: platform, isLoading } = useQuery({
    queryKey: ['platform'],
    queryFn: () => apiClient.get<Platform>('/platform').then(r => r.data),
  });

  const { data: components } = useQuery({
    queryKey: ['platform-components'],
    queryFn: () => apiClient.get<Record<string, { phase: string; endpoint: string }>>('/platform/components').then(r => r.data),
  });

  if (isLoading) return <Spin size="large" />;

  const componentList: Component[] = components
    ? Object.entries(components).map(([name, c]) => ({ name, phase: c.phase, endpoint: c.endpoint }))
    : [];

  return (
    <div>
      <Title level={3}>Platform Status</Title>

      <Card title="Platform" style={{ marginBottom: 16 }}>
        <Descriptions column={2}>
          <Descriptions.Item label="Name">{platform?.metadata?.name ?? 'N/A'}</Descriptions.Item>
          <Descriptions.Item label="Namespace">{platform?.metadata?.namespace ?? 'N/A'}</Descriptions.Item>
          <Descriptions.Item label="Phase">
            <Tag color={platform?.status?.phase === 'Ready' ? 'success' : 'processing'}>
              {platform?.status?.phase ?? 'Unknown'}
            </Tag>
          </Descriptions.Item>
        </Descriptions>
      </Card>

      <Card title="Components">
        <Table
          dataSource={componentList}
          rowKey="name"
          columns={[
            { title: 'Component', dataIndex: 'name', render: (n: string) => <span style={{ textTransform: 'capitalize' }}>{n}</span> },
            { title: 'Status', dataIndex: 'phase',
              render: (p: string) => <Tag color={p === 'Ready' || p === 'Running' ? 'success' : p === 'Unknown' ? 'default' : 'warning'}>{p}</Tag> },
            { title: 'Endpoint', dataIndex: 'endpoint', ellipsis: true },
          ]}
          pagination={false}
        />
      </Card>
    </div>
  );
};

export default PlatformStatus;
