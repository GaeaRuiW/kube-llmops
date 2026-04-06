import React from 'react';
import { Table, Button, Tag, Space, Typography, Popconfirm, message } from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import apiClient from '../../api/client';
import { PermissionGuard } from '../../components/PermissionGuard';
import { UserForm } from './UserForm';
import type { User } from '../../types';

const { Title } = Typography;

const UserList: React.FC = () => {
  const qc = useQueryClient();
  const [formOpen, setFormOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<User | null>(null);

  const { data: users, isLoading } = useQuery({
    queryKey: ['users'],
    queryFn: () => apiClient.get<User[]>('/users').then(r => r.data),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiClient.delete(`/users/${id}`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['users'] }); message.success('Deleted'); },
  });

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}>Users</Title>
        <PermissionGuard resource="users" action="create">
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); setFormOpen(true); }}>
            Add User
          </Button>
        </PermissionGuard>
      </div>
      <Table
        columns={[
          { title: 'Name', dataIndex: 'displayName' },
          { title: 'Email', dataIndex: 'email' },
          { title: 'Roles', dataIndex: 'roles', render: (roles: User['roles']) =>
            roles?.map(r => <Tag key={r.id} color={r.isSystem ? 'blue' : 'default'}>{r.name}</Tag>) ?? '-' },
          { title: 'Enabled', dataIndex: 'enabled', render: (e: boolean) => <Tag color={e ? 'success' : 'error'}>{e ? 'Yes' : 'No'}</Tag> },
          { title: 'Actions', render: (_: unknown, u: User) => (
            <Space>
              <PermissionGuard resource="users" action="edit">
                <Button type="text" icon={<EditOutlined />} onClick={() => { setEditing(u); setFormOpen(true); }} />
              </PermissionGuard>
              <PermissionGuard resource="users" action="delete">
                <Popconfirm title="Delete user?" onConfirm={() => deleteMutation.mutate(u.id)}>
                  <Button type="text" danger icon={<DeleteOutlined />} />
                </Popconfirm>
              </PermissionGuard>
            </Space>
          )},
        ]}
        dataSource={users ?? []}
        rowKey="id"
        loading={isLoading}
      />
      <UserForm open={formOpen} user={editing} onClose={() => setFormOpen(false)} />
    </div>
  );
};

export default UserList;
