import React from 'react';
import { Table, Button, Tag, Space, Typography, Popconfirm, message } from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import apiClient from '../../api/client';
import { PermissionGuard } from '../../components/PermissionGuard';
import { PermissionForm } from './PermissionForm';
import type { Permission } from '../../types';

const { Title } = Typography;

const PermissionList: React.FC = () => {
  const qc = useQueryClient();
  const [formOpen, setFormOpen] = React.useState(false);

  const { data: perms, isLoading } = useQuery({
    queryKey: ['permissions'],
    queryFn: () => apiClient.get<Permission[]>('/permissions').then(r => r.data),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiClient.delete(`/permissions/${id}`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['permissions'] }); message.success('Deleted'); },
  });

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}>Permissions</Title>
        <PermissionGuard resource="permissions" action="create">
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setFormOpen(true)}>Add Permission</Button>
        </PermissionGuard>
      </div>
      <Table
        columns={[
          { title: 'Resource', dataIndex: 'resource' },
          { title: 'Action', dataIndex: 'action' },
          { title: 'Description', dataIndex: 'description', ellipsis: true },
          { title: 'System', dataIndex: 'isSystem', render: (s: boolean) => s ? <Tag color="blue">System</Tag> : null },
          { title: 'Actions', render: (_: unknown, p: Permission) => !p.isSystem ? (
            <Popconfirm title="Delete?" onConfirm={() => deleteMutation.mutate(p.id)}>
              <Button type="text" danger icon={<DeleteOutlined />} />
            </Popconfirm>
          ) : null },
        ]}
        dataSource={perms ?? []}
        rowKey="id"
        loading={isLoading}
      />
      <PermissionForm open={formOpen} onClose={() => setFormOpen(false)} />
    </div>
  );
};

export default PermissionList;
