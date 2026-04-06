import React from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, Descriptions, Tag, Button, Space, Typography, Spin, Timeline } from 'antd';
import { ArrowLeftOutlined, CheckCircleOutlined, CloseCircleOutlined, LoadingOutlined, ClockCircleOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import apiClient from '../../api/client';
import type { FineTuneRun } from '../../types';

const { Title, Text } = Typography;

const FinetuneDetail: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const navigate = useNavigate();

  const { data: run, isLoading } = useQuery({
    queryKey: ['finetune', name],
    queryFn: () => apiClient.get<FineTuneRun>(`/finetune/${name}`).then(r => r.data),
    enabled: !!name,
    refetchInterval: 5000,
  });

  if (isLoading) return <Spin size="large" />;
  if (!run) return <div>Not found</div>;

  const pipelineSteps = ['prepare-data', 'finetune', 'merge-upload', 'evaluate', 'quality-gate', 'deploy'];
  const phase = run.status?.phase ?? 'Pending';

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/finetune')}>Back</Button>
        <Title level={3} style={{ margin: 0 }}>{name}</Title>
        <Tag color={phase === 'Completed' ? 'success' : phase === 'Running' ? 'processing' : phase === 'Failed' ? 'error' : 'default'}>{phase}</Tag>
      </Space>

      <Card title="Configuration" style={{ marginBottom: 16 }}>
        <Descriptions column={2}>
          <Descriptions.Item label="Base Model">{run.spec.baseModel}</Descriptions.Item>
          <Descriptions.Item label="Output">{run.spec.outputName}</Descriptions.Item>
          <Descriptions.Item label="Method">{run.spec.method}</Descriptions.Item>
          <Descriptions.Item label="Data Source">{run.spec.dataSource.type}: {run.spec.dataSource.path}</Descriptions.Item>
          <Descriptions.Item label="Epochs">{run.spec.training.epochs}</Descriptions.Item>
          <Descriptions.Item label="Batch Size">{run.spec.training.batchSize}</Descriptions.Item>
          <Descriptions.Item label="Learning Rate">{run.spec.training.learningRate}</Descriptions.Item>
          <Descriptions.Item label="GPU">{run.spec.resources.gpu}</Descriptions.Item>
        </Descriptions>
      </Card>

      {run.status?.metrics && (
        <Card title="Metrics" style={{ marginBottom: 16 }}>
          <Descriptions column={3}>
            <Descriptions.Item label="Train Loss">{run.status.metrics.trainLoss}</Descriptions.Item>
            <Descriptions.Item label="Eval Loss">{run.status.metrics.evalLoss}</Descriptions.Item>
            <Descriptions.Item label="Duration">{run.status.metrics.trainingDuration}</Descriptions.Item>
          </Descriptions>
        </Card>
      )}

      {run.status?.qualityGate && (
        <Card title="Quality Gate" style={{ marginBottom: 16 }}>
          <Tag color={run.status.qualityGate.passed ? 'success' : 'error'} icon={run.status.qualityGate.passed ? <CheckCircleOutlined /> : <CloseCircleOutlined />}>
            {run.status.qualityGate.passed ? 'PASSED' : 'FAILED'}
          </Tag>
          <Text style={{ marginLeft: 8 }}>{run.status.qualityGate.message}</Text>
        </Card>
      )}

      <Card title="Pipeline">
        <Timeline items={pipelineSteps.map((step) => ({
          children: step,
          color: phase === 'Completed' ? 'green' : 'gray',
          dot: phase === 'Completed' ? <CheckCircleOutlined /> :
               phase === 'Running' ? <LoadingOutlined /> :
               <ClockCircleOutlined />,
        }))} />
      </Card>
    </div>
  );
};

export default FinetuneDetail;
