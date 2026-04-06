import React from 'react';
import { Table, Button, Tag, Typography, Popconfirm, message, Modal, Form, Input } from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import apiClient from '../../api/client';
import { PermissionGuard } from '../../components/PermissionGuard';

const { Title } = Typography;

interface KnowledgeBase {
  id: string;
  name: string;
  description: string;
  documentCount: number;
  status: string;
}

const RagList: React.FC = () => {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [createOpen, setCreateOpen] = React.useState(false);
  const [form] = Form.useForm();

  const { data: kbs, isLoading } = useQuery({
    queryKey: ['rag-kbs'],
    queryFn: () => apiClient.get<KnowledgeBase[]>('/rag').then(r => r.data),
  });

  const createMutation = useMutation({
    mutationFn: (values: { name: string; description: string }) => apiClient.post('/rag', values),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['rag-kbs'] }); setCreateOpen(false); form.resetFields(); message.success('Created'); },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiClient.delete(`/rag/${id}`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['rag-kbs'] }); message.success('Deleted'); },
  });

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}>Knowledge Bases</Title>
        <PermissionGuard resource="rag" action="create">
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>New KB</Button>
        </PermissionGuard>
      </div>
      <Table
        columns={[
          { title: 'Name', dataIndex: 'name', render: (name: string, r: KnowledgeBase) => <a onClick={() => navigate(`/rag/${r.id}`)}>{name}</a> },
          { title: 'Description', dataIndex: 'description', ellipsis: true },
          { title: 'Documents', dataIndex: 'documentCount' },
          { title: 'Status', dataIndex: 'status', render: (s: string) => <Tag color={s === 'ready' ? 'success' : 'processing'}>{s}</Tag> },
          { title: 'Actions', render: (_: unknown, r: KnowledgeBase) => (
            <Popconfirm title="Delete?" onConfirm={() => deleteMutation.mutate(r.id)}>
              <Button type="text" danger icon={<DeleteOutlined />} />
            </Popconfirm>
          )},
        ]}
        dataSource={kbs ?? []}
        rowKey="id"
        loading={isLoading}
      />
      <Modal title="Create Knowledge Base" open={createOpen} onOk={() => form.validateFields().then(v => createMutation.mutate(v))}
        onCancel={() => setCreateOpen(false)} confirmLoading={createMutation.isPending}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="Name" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="description" label="Description"><Input.TextArea /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default RagList;
