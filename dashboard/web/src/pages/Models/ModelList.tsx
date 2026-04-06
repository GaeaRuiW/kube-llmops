import React from 'react';
import { Table, Button, Tag, Space, Dropdown, Typography, Popconfirm, InputNumber, Modal, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { PlusOutlined, MoreOutlined, ScissorOutlined, DeleteOutlined, ExpandOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import apiClient from '../../api/client';
import { PermissionGuard } from '../../components/PermissionGuard';
import type { ModelDeployment } from '../../types';

const { Title } = Typography;

const phaseColor: Record<string, string> = {
  Ready: 'success', Progressing: 'processing', Failed: 'error', Unknown: 'default',
};

const ModelList: React.FC = () => {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [scaleModal, setScaleModal] = React.useState<{ name: string; replicas: number } | null>(null);

  const { data: models, isLoading } = useQuery({
    queryKey: ['models'],
    queryFn: () => apiClient.get<ModelDeployment[]>('/models').then(r => r.data),
  });

  const deleteMutation = useMutation({
    mutationFn: (name: string) => apiClient.delete(`/models/${name}`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['models'] }); message.success('Model deleted'); },
  });

  const scaleMutation = useMutation({
    mutationFn: ({ name, replicas }: { name: string; replicas: number }) =>
      apiClient.post(`/models/${name}/scale`, { replicas }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['models'] }); setScaleModal(null); message.success('Scaled'); },
  });

  const columns: ColumnsType<ModelDeployment> = [
    { title: 'Name', dataIndex: ['metadata', 'name'], key: 'name',
      render: (name: string) => <a onClick={() => navigate(`/models/${name}`)}>{name}</a> },
    { title: 'Engine', dataIndex: ['status', 'engine'], key: 'engine', render: (e: string) => <Tag>{e}</Tag> },
    { title: 'Source', dataIndex: ['spec', 'source'], key: 'source', ellipsis: true },
    { title: 'Status', dataIndex: ['status', 'phase'], key: 'phase',
      render: (p: string) => <Tag color={phaseColor[p] || 'default'}>{p}</Tag> },
    { title: 'Replicas', key: 'replicas',
      render: (_: unknown, r: ModelDeployment) => `${r.status?.readyReplicas ?? 0}/${r.status?.totalReplicas ?? r.spec.replicas ?? 1}` },
    { title: 'GPU', dataIndex: ['spec', 'resources', 'gpu'], key: 'gpu' },
    { title: 'Actions', key: 'actions', render: (_: unknown, record: ModelDeployment) => (
      <Dropdown menu={{ items: [
        { key: 'scale', icon: <ExpandOutlined />, label: 'Scale',
          onClick: () => setScaleModal({ name: record.metadata.name, replicas: record.spec.replicas ?? 1 }) },
        { key: 'canary', icon: <ScissorOutlined />, label: 'Canary', onClick: () => navigate(`/models/${record.metadata.name}`) },
        { type: 'divider' as const },
        { key: 'delete', icon: <DeleteOutlined />, label: 'Delete', danger: true,
          onClick: () => deleteMutation.mutate(record.metadata.name) },
      ]}}>
        <Button type="text" icon={<MoreOutlined />} />
      </Dropdown>
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}>Models</Title>
        <PermissionGuard resource="models" action="create">
          <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/models/deploy')}>
            Deploy Model
          </Button>
        </PermissionGuard>
      </div>
      <Table columns={columns} dataSource={models ?? []} rowKey={r => r.metadata.name} loading={isLoading} />
      <Modal title="Scale Model" open={!!scaleModal} onOk={() => scaleModal && scaleMutation.mutate(scaleModal)}
        onCancel={() => setScaleModal(null)} confirmLoading={scaleMutation.isPending}>
        <p>Replicas:</p>
        <InputNumber min={0} max={10} value={scaleModal?.replicas}
          onChange={v => scaleModal && setScaleModal({ ...scaleModal, replicas: v ?? 1 })} />
      </Modal>
    </div>
  );
};

export default ModelList;
