import React from 'react';
import { Table, Button, Tag, Space, Typography, Popconfirm, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import apiClient from '../../api/client';
import { PermissionGuard } from '../../components/PermissionGuard';
import type { FineTuneRun } from '../../types';

const { Title } = Typography;

const phaseColor: Record<string, string> = {
  Completed: 'success', Running: 'processing', Failed: 'error', Pending: 'default',
};

const FinetuneList: React.FC = () => {
  const navigate = useNavigate();
  const qc = useQueryClient();

  const { data: runs, isLoading } = useQuery({
    queryKey: ['finetunes'],
    queryFn: () => apiClient.get<FineTuneRun[]>('/finetune').then(r => r.data),
  });

  const deleteMutation = useMutation({
    mutationFn: (name: string) => apiClient.delete(`/finetune/${name}`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['finetunes'] }); message.success('Deleted'); },
  });

  const columns: ColumnsType<FineTuneRun> = [
    { title: 'Name', dataIndex: ['metadata', 'name'],
      render: (name: string) => <a onClick={() => navigate(`/finetune/${name}`)}>{name}</a> },
    { title: 'Base Model', dataIndex: ['spec', 'baseModel'], ellipsis: true },
    { title: 'Method', dataIndex: ['spec', 'method'], render: (m: string) => <Tag>{m}</Tag> },
    { title: 'Status', dataIndex: ['status', 'phase'],
      render: (p: string) => <Tag color={phaseColor[p] || 'default'}>{p ?? 'Unknown'}</Tag> },
    { title: 'QG', dataIndex: ['status', 'qualityGate', 'passed'],
      render: (p: boolean | undefined) => p === undefined ? '-' : <Tag color={p ? 'success' : 'error'}>{p ? 'Pass' : 'Fail'}</Tag> },
    { title: 'Actions', render: (_: unknown, r: FineTuneRun) => (
      <Popconfirm title="Delete?" onConfirm={() => deleteMutation.mutate(r.metadata.name)}>
        <Button type="text" danger icon={<DeleteOutlined />} />
      </Popconfirm>
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}>Fine-tuning</Title>
        <PermissionGuard resource="finetune" action="create">
          <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/finetune/create')}>
            New Fine-tune
          </Button>
        </PermissionGuard>
      </div>
      <Table columns={columns} dataSource={runs ?? []} rowKey={r => r.metadata.name} loading={isLoading} />
    </div>
  );
};

export default FinetuneList;
