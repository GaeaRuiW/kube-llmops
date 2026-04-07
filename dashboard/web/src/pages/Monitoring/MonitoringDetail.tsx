import React from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Button, Space, Typography } from 'antd';
import { ArrowLeftOutlined } from '@ant-design/icons';
import { IframeEmbed } from '../../components/IframeEmbed';

const { Title } = Typography;

const dashboardTitles: Record<string, string> = {
  'vllm-overview': 'vLLM Model Serving',
  'litellm-gateway': 'LiteLLM AI Gateway',
  'gpu-overview': 'GPU & Infrastructure',
  'rag-quality': 'RAG Quality (Ragas)',
  'cost-usage': 'Cost & Usage',
  'slo-overview': 'SLO Overview',
  'infra-roi': 'Infrastructure ROI',
  'tenant-overview': 'Tenant Overview',
  'milvus-overview': 'Milvus Vector DB',
  'system-overview': 'System Overview',
  'finetune-overview': 'Fine-tuning Pipeline',
};

const MonitoringDetail: React.FC = () => {
  const { uid } = useParams<{ uid: string }>();
  const navigate = useNavigate();
  const title = dashboardTitles[uid ?? ''] ?? uid ?? 'Dashboard';

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/monitoring')}>Back</Button>
        <Title level={3} style={{ margin: 0 }}>{title}</Title>
      </Space>
      <IframeEmbed src={`/services/grafana/d/${uid}/?kiosk`} title={title} />
    </div>
  );
};

export default MonitoringDetail;
