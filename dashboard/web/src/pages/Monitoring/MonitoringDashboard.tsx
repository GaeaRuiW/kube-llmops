import React from 'react';
import { Row, Col, Card, Typography, Tag, List } from 'antd';
import { LineChartOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';

const { Title } = Typography;

const dashboards = [
  { uid: 'vllm-overview', title: 'vLLM Model Serving', desc: 'Request latency, throughput, GPU utilization' },
  { uid: 'litellm-gateway', title: 'LiteLLM AI Gateway', desc: 'Routing, costs, token usage' },
  { uid: 'gpu-overview', title: 'GPU & Infrastructure', desc: 'GPU memory, utilization, temperature' },
  { uid: 'rag-quality', title: 'RAG Quality (Ragas)', desc: 'Faithfulness, relevancy, precision, recall' },
  { uid: 'cost-usage', title: 'Cost & Usage', desc: 'Per-model cost tracking' },
  { uid: 'slo-overview', title: 'SLO Overview', desc: 'Service level objectives' },
  { uid: 'infra-roi', title: 'Infrastructure ROI', desc: 'Cost efficiency metrics' },
  { uid: 'tenant-overview', title: 'Tenant Overview', desc: 'Multi-tenant usage' },
  { uid: 'milvus-overview', title: 'Milvus Vector DB', desc: 'Vector search metrics' },
  { uid: 'system-overview', title: 'System Overview', desc: 'CPU, memory, disk, network' },
  { uid: 'finetune-overview', title: 'Fine-tuning Pipeline', desc: 'Training progress, loss curves' },
];

const otherTools = [
  { name: 'Langfuse', desc: 'LLM Tracing & Analytics', path: '/services/langfuse' },
  { name: 'MLflow', desc: 'Experiment Tracking & Model Registry', path: '/services/mlflow' },
];

const MonitoringDashboard: React.FC = () => {
  const navigate = useNavigate();

  return (
    <div>
      <Title level={3}>Monitoring</Title>

      <Title level={5}>Grafana Dashboards</Title>
      <Row gutter={[16, 16]}>
        {dashboards.map(d => (
          <Col xs={24} sm={12} lg={8} key={d.uid}>
            <Card
              hoverable
              onClick={() => navigate(`/services/grafana`)}
              size="small"
            >
              <Card.Meta
                avatar={<LineChartOutlined style={{ fontSize: 24 }} />}
                title={d.title}
                description={d.desc}
              />
            </Card>
          </Col>
        ))}
      </Row>

      <Title level={5} style={{ marginTop: 24 }}>Observability Tools</Title>
      <List
        dataSource={otherTools}
        renderItem={tool => (
          <List.Item style={{ cursor: 'pointer' }} onClick={() => navigate(tool.path)}>
            <List.Item.Meta title={tool.name} description={tool.desc} />
            <Tag color="blue">Open</Tag>
          </List.Item>
        )}
      />
    </div>
  );
};

export default MonitoringDashboard;
