import React from 'react';
import { Table, Button, Tag, Space, Typography, Popconfirm, message } from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import apiClient from '../../api/client';
import { PermissionGuard } from '../../components/PermissionGuard';
import { RoleForm } from './RoleForm';
import type { Role } from '../../types';

const { Title } = Typography;

const RoleList: React.FC = () => {
  const qc = useQueryClient();
  const [formOpen, setFormOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<Role | null>(null);

  const { data: roles, isLoading } = useQuery({
    queryKey: ['roles'],
    queryFn: () => apiClient.get<Role[]>('/roles').then(r => r.data),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiClient.delete(`/roles/${id}`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['roles'] }); message.success('Deleted'); },
  });

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}>Roles</Title>
        <PermissionGuard resource="roles" action="create">
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); setFormOpen(true); }}>
            Create Role
          </Button>
        </PermissionGuard>
      </div>
      <Table
        columns={[
          { title: 'Name', dataIndex: 'name' },
          { title: 'Description', dataIndex: 'description', ellipsis: true },
          { title: 'System', dataIndex: 'isSystem', render: (s: boolean) => s ? <Tag color="blue">System</Tag> : null },
          { title: 'Permissions', dataIndex: 'permissions', render: (p: Role['permissions']) => p?.length ?? 0 },
          { title: 'Actions', render: (_: unknown, r: Role) => (
            <Space>
              {!r.isSystem && (
                <>
                  <PermissionGuard resource="roles" action="edit">
                    <Button type="text" icon={<EditOutlined />} onClick={() => { setEditing(r); setFormOpen(true); }} />
                  </PermissionGuard>
                  <PermissionGuard resource="roles" action="delete">
                    <Popconfirm title="Delete?" onConfirm={() => deleteMutation.mutate(r.id)}>
                      <Button type="text" danger icon={<DeleteOutlined />} />
                    </Popconfirm>
                  </PermissionGuard>
                </>
              )}
            </Space>
          )},
        ]}
        dataSource={roles ?? []}
        rowKey="id"
        loading={isLoading}
      />
      <RoleForm open={formOpen} role={editing} onClose={() => setFormOpen(false)} />
    </div>
  );
};

export default RoleList;
