import React from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, Button, Space, Typography, Upload, Input, List, message, Spin } from 'antd';
import { ArrowLeftOutlined, UploadOutlined, SendOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import apiClient from '../../api/client';

const { Title } = Typography;
const { TextArea } = Input;

const RagDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [query, setQuery] = React.useState('');
  const [results, setResults] = React.useState<Array<{ content: string; score: number }>>([]);

  const { data: kb, isLoading } = useQuery({
    queryKey: ['rag-kb', id],
    queryFn: () => apiClient.get(`/rag/${id}`).then(r => r.data),
    enabled: !!id,
  });

  const uploadMutation = useMutation({
    mutationFn: (file: File) => {
      const formData = new FormData();
      formData.append('file', file);
      return apiClient.post(`/rag/${id}/upload`, formData, { headers: { 'Content-Type': 'multipart/form-data' } });
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['rag-kb', id] }); message.success('Uploaded'); },
  });

  const queryMutation = useMutation({
    mutationFn: (q: string) => apiClient.post(`/rag/${id}/query`, { query: q }).then(r => r.data),
    onSuccess: (data) => setResults(data.results ?? []),
  });

  if (isLoading) return <Spin size="large" />;

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/rag')}>Back</Button>
        <Title level={3} style={{ margin: 0 }}>{kb?.name ?? id}</Title>
      </Space>

      <Card title="Upload Documents" style={{ marginBottom: 16 }}>
        <Upload beforeUpload={(file) => { uploadMutation.mutate(file); return false; }} showUploadList={false}>
          <Button icon={<UploadOutlined />} loading={uploadMutation.isPending}>Upload File</Button>
        </Upload>
      </Card>

      <Card title="Query Test" style={{ marginBottom: 16 }}>
        <Space.Compact style={{ width: '100%' }}>
          <TextArea rows={2} value={query} onChange={e => setQuery(e.target.value)} placeholder="Ask a question..." />
          <Button type="primary" icon={<SendOutlined />} loading={queryMutation.isPending}
            onClick={() => query && queryMutation.mutate(query)} style={{ height: 'auto' }}>
            Query
          </Button>
        </Space.Compact>
        {results.length > 0 && (
          <List style={{ marginTop: 16 }} dataSource={results} renderItem={(r, i) => (
            <List.Item>
              <List.Item.Meta title={`Result ${i + 1} (score: ${r.score})`} description={r.content} />
            </List.Item>
          )} />
        )}
      </Card>
    </div>
  );
};

export default RagDetail;
