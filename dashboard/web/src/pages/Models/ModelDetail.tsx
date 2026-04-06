import React from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, Descriptions, Tag, Table, Button, Space, Typography, Spin, List } from 'antd';
import { ArrowLeftOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import apiClient from '../../api/client';
import type { ModelDeployment } from '../../types';

const { Title, Text } = Typography;

const ModelDetail: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const navigate = useNavigate();

  const { data: model, isLoading } = useQuery({
    queryKey: ['model', name],
    queryFn: () => apiClient.get<ModelDeployment>(`/models/${name}`).then(r => r.data),
    enabled: !!name,
  });

  const { data: pods } = useQuery({
    queryKey: ['model-pods', name],
    queryFn: () => apiClient.get<Array<{ name: string; phase: string; node: string; ready: boolean }>>(`/models/${name}/pods`).then(r => r.data),
    enabled: !!name,
  });

  if (isLoading) return <Spin size="large" />;
  if (!model) return <div>Model not found</div>;

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/models')}>Back</Button>
        <Title level={3} style={{ margin: 0 }}>{name}</Title>
        <Tag color={model.status?.phase === 'Ready' ? 'success' : 'processing'}>{model.status?.phase}</Tag>
      </Space>

      <Card title="Specification" style={{ marginBottom: 16 }}>
        <Descriptions column={2}>
          <Descriptions.Item label="Source">{model.spec.source}</Descriptions.Item>
          <Descriptions.Item label="Engine">{model.status?.engine || model.spec.engine}</Descriptions.Item>
          <Descriptions.Item label="Replicas">{model.status?.readyReplicas ?? 0}/{model.status?.totalReplicas ?? model.spec.replicas ?? 1}</Descriptions.Item>
          <Descriptions.Item label="Endpoint">{model.status?.endpoint || 'N/A'}</Descriptions.Item>
          <Descriptions.Item label="GPU">{model.spec.resources.gpu}</Descriptions.Item>
          <Descriptions.Item label="Memory">{model.spec.resources.memory}</Descriptions.Item>
        </Descriptions>
      </Card>

      {model.status?.canary && (
        <Card title="Canary" style={{ marginBottom: 16 }}>
          <Descriptions column={2}>
            <Descriptions.Item label="Source">{model.spec.canary?.source}</Descriptions.Item>
            <Descriptions.Item label="Weight">{model.spec.canary?.weight}%</Descriptions.Item>
            <Descriptions.Item label="Phase">{model.status.canary.phase}</Descriptions.Item>
            <Descriptions.Item label="Ready">{model.status.canary.readyReplicas}</Descriptions.Item>
          </Descriptions>
        </Card>
      )}

      {model.status?.conditions && model.status.conditions.length > 0 && (
        <Card title="Conditions" style={{ marginBottom: 16 }}>
          <List dataSource={model.status.conditions} renderItem={c => (
            <List.Item>
              <Tag color={c.status === 'True' ? 'success' : 'warning'}>{c.type}</Tag>
              <Text>{c.message}</Text>
            </List.Item>
          )} />
        </Card>
      )}

      <Card title="Pods">
        <Table
          dataSource={pods ?? []}
          rowKey="name"
          columns={[
            { title: 'Name', dataIndex: 'name' },
            { title: 'Phase', dataIndex: 'phase', render: (p: string) => <Tag color={p === 'Running' ? 'success' : 'warning'}>{p}</Tag> },
            { title: 'Node', dataIndex: 'node' },
            { title: 'Ready', dataIndex: 'ready', render: (r: boolean) => <Tag color={r ? 'success' : 'error'}>{r ? 'Yes' : 'No'}</Tag> },
          ]}
        />
      </Card>
    </div>
  );
};

export default ModelDetail;
