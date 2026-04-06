import React from 'react';
import { Row, Col, Card, Tag, Typography, Spin } from 'antd';
import {
  DashboardOutlined, SearchOutlined, RobotOutlined, ExperimentOutlined,
  CodeOutlined, DatabaseOutlined, LockOutlined, ApiOutlined, BarChartOutlined,
} from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import apiClient from '../../api/client';
import type { ServiceInfo } from '../../types';

const { Title, Text } = Typography;

const iconMap: Record<string, React.ReactNode> = {
  dashboard: <DashboardOutlined />, search: <SearchOutlined />, robot: <RobotOutlined />,
  experiment: <ExperimentOutlined />, code: <CodeOutlined />, database: <DatabaseOutlined />,
  lock: <LockOutlined />, api: <ApiOutlined />, 'bar-chart': <BarChartOutlined />,
};

const ServiceGrid: React.FC = () => {
  const navigate = useNavigate();

  const { data: services, isLoading } = useQuery({
    queryKey: ['services'],
    queryFn: () => apiClient.get<ServiceInfo[]>('/services').then(r => r.data),
  });

  if (isLoading) return <Spin size="large" />;

  return (
    <div>
      <Title level={3}>Services</Title>
      <Row gutter={[16, 16]}>
        {(services ?? []).map(svc => (
          <Col xs={24} sm={12} lg={8} xl={6} key={svc.name}>
            <Card
              hoverable
              onClick={() => navigate(`/services/${svc.name}`)}
              style={{ textAlign: 'center' }}
            >
              <div style={{ fontSize: 32, marginBottom: 8 }}>{iconMap[svc.icon] || <ApiOutlined />}</div>
              <Title level={5} style={{ margin: 0 }}>{svc.name}</Title>
              <Text type="secondary">{svc.description}</Text>
              <div style={{ marginTop: 8 }}>
                <Tag color={svc.phase === 'Ready' || svc.phase === 'Running' ? 'success' : svc.phase === 'Unknown' ? 'default' : 'warning'}>
                  {svc.phase}
                </Tag>
              </div>
            </Card>
          </Col>
        ))}
      </Row>
    </div>
  );
};

export default ServiceGrid;
