import React, { useEffect } from 'react';
import { Modal, Form, Input, Select, Switch, message } from 'antd';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import apiClient from '../../api/client';
import type { User, Role } from '../../types';

interface Props {
  open: boolean;
  user: User | null;
  onClose: () => void;
}

export const UserForm: React.FC<Props> = ({ open, user, onClose }) => {
  const [form] = Form.useForm();
  const qc = useQueryClient();

  const { data: roles } = useQuery({
    queryKey: ['roles'],
    queryFn: () => apiClient.get<Role[]>('/roles').then(r => r.data),
  });

  useEffect(() => {
    if (open) {
      form.setFieldsValue(user ? {
        email: user.email, displayName: user.displayName, enabled: user.enabled,
        roleIds: user.roles?.map(r => r.id) ?? [],
      } : { enabled: true, roleIds: [] });
    }
  }, [open, user, form]);

  const createMutation = useMutation({
    mutationFn: (values: Record<string, unknown>) => apiClient.post('/users', values),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['users'] }); message.success('Created'); onClose(); },
  });

  const updateMutation = useMutation({
    mutationFn: (values: Record<string, unknown>) => apiClient.put(`/users/${user!.id}`, values),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['users'] }); message.success('Updated'); onClose(); },
  });

  const onOk = () => {
    form.validateFields().then(values => {
      if (user) { updateMutation.mutate(values); }
      else { createMutation.mutate(values); }
    });
  };

  return (
    <Modal title={user ? 'Edit User' : 'Create User'} open={open} onOk={onOk} onCancel={onClose}
      confirmLoading={createMutation.isPending || updateMutation.isPending}>
      <Form form={form} layout="vertical">
        <Form.Item name="email" label="Email" rules={[{ required: true, type: 'email' }]}>
          <Input disabled={!!user} />
        </Form.Item>
        <Form.Item name="displayName" label="Display Name" rules={[{ required: true }]}>
          <Input />
        </Form.Item>
        <Form.Item name="enabled" label="Enabled" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="roleIds" label="Roles">
          <Select mode="multiple" placeholder="Select roles"
            options={roles?.map(r => ({ value: r.id, label: r.name })) ?? []} />
        </Form.Item>
      </Form>
    </Modal>
  );
};
