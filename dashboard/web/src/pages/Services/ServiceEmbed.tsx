import React from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Button, Space, Typography } from 'antd';
import { ArrowLeftOutlined } from '@ant-design/icons';
import { IframeEmbed } from '../../components/IframeEmbed';

const { Title } = Typography;

const ServiceEmbed: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const navigate = useNavigate();

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/services')}>Back</Button>
        <Title level={3} style={{ margin: 0, textTransform: 'capitalize' }}>{name}</Title>
      </Space>
      <IframeEmbed src={`/services/${name}/`} title={name} />
    </div>
  );
};

export default ServiceEmbed;
