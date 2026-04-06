import React from 'react';
import { Row, Col, Card, Statistic, Tag, List, Typography, Spin } from 'antd';
import { CloudServerOutlined, ExperimentOutlined, CheckCircleOutlined, WarningOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import apiClient from '../../api/client';
import type { ModelDeployment, FineTuneRun, ServiceInfo } from '../../types';

const { Title, Text } = Typography;

const Overview: React.FC = () => {
  const { data: models, isLoading: modelsLoading } = useQuery({
    queryKey: ['models'],
    queryFn: () => apiClient.get<ModelDeployment[]>('/models').then(r => r.data),
  });

  const { data: finetunes } = useQuery({
    queryKey: ['finetunes'],
    queryFn: () => apiClient.get<FineTuneRun[]>('/finetune').then(r => r.data),
  });

  const { data: services } = useQuery({
    queryKey: ['services'],
    queryFn: () => apiClient.get<ServiceInfo[]>('/services').then(r => r.data),
  });

  const readyModels = models?.filter(m => m.status?.phase === 'Ready').length ?? 0;
  const totalModels = models?.length ?? 0;
  const runningFt = finetunes?.filter(f => f.status?.phase === 'Running').length ?? 0;
  const healthyServices = services?.filter(s => s.phase === 'Ready' || s.phase === 'Running').length ?? 0;
  const totalServices = services?.length ?? 0;

  return (
    <div>
      <Title level={3}>Overview</Title>
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic title="Models" value={totalModels} prefix={<CloudServerOutlined />}
              suffix={<Text type="secondary">({readyModels} ready)</Text>} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic title="Fine-tune Jobs" value={finetunes?.length ?? 0} prefix={<ExperimentOutlined />}
              suffix={runningFt > 0 ? <Tag color="processing">{runningFt} running</Tag> : null} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic title="Services" value={totalServices} prefix={<CheckCircleOutlined />}
              suffix={<Text type="secondary">({healthyServices} healthy)</Text>} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic title="Alerts" value={0} prefix={<WarningOutlined />}
              suffix={<Tag color="success">All clear</Tag>} />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} lg={12}>
          <Card title="Model Status" loading={modelsLoading}>
            <List
              dataSource={models?.slice(0, 5) ?? []}
              renderItem={(m) => (
                <List.Item>
                  <List.Item.Meta
                    title={m.metadata.name}
                    description={`${m.spec.engine} · ${m.spec.source}`}
                  />
                  <Tag color={m.status?.phase === 'Ready' ? 'success' : m.status?.phase === 'Progressing' ? 'processing' : 'warning'}>
                    {m.status?.phase ?? 'Unknown'}
                  </Tag>
                </List.Item>
              )}
              locale={{ emptyText: 'No models deployed' }}
            />
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title="Service Health">
            <List
              dataSource={services ?? []}
              renderItem={(s) => (
                <List.Item>
                  <List.Item.Meta title={s.name} description={s.description} />
                  <Tag color={s.phase === 'Ready' || s.phase === 'Running' ? 'success' : 'warning'}>
                    {s.phase}
                  </Tag>
                </List.Item>
              )}
              locale={{ emptyText: 'No services' }}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Overview;
