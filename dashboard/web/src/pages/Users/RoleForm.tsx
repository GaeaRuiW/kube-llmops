import React, { useEffect, useMemo } from 'react';
import { Modal, Form, Input, Checkbox, Typography, message } from 'antd';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import apiClient from '../../api/client';
import type { Role, Permission } from '../../types';

const { Text } = Typography;

interface Props {
  open: boolean;
  role: Role | null;
  onClose: () => void;
}

export const RoleForm: React.FC<Props> = ({ open, role, onClose }) => {
  const [form] = Form.useForm();
  const qc = useQueryClient();

  const { data: allPerms } = useQuery({
    queryKey: ['permissions'],
    queryFn: () => apiClient.get<Permission[]>('/permissions').then(r => r.data),
  });

  // Group permissions by resource
  const grouped = useMemo(() => {
    const map = new Map<string, Permission[]>();
    (allPerms ?? []).forEach(p => {
      const list = map.get(p.resource) ?? [];
      list.push(p);
      map.set(p.resource, list);
    });
    return map;
  }, [allPerms]);

  useEffect(() => {
    if (open) {
      form.setFieldsValue(role ? {
        name: role.name, description: role.description,
        permissionIds: role.permissions?.map(p => p.id) ?? [],
      } : { permissionIds: [] });
    }
  }, [open, role, form]);

  const createMutation = useMutation({
    mutationFn: (values: Record<string, unknown>) => apiClient.post('/roles', values),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['roles'] }); message.success('Created'); onClose(); },
  });

  const updateMutation = useMutation({
    mutationFn: async (values: Record<string, unknown>) => {
      if (role) {
        await apiClient.put(`/roles/${role.id}`, { name: values.name, description: values.description });
        await apiClient.put(`/roles/${role.id}/permissions`, { permissionIds: values.permissionIds });
      }
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['roles'] }); message.success('Updated'); onClose(); },
  });

  const onOk = () => {
    form.validateFields().then(values => {
      if (role) { updateMutation.mutate(values); }
      else { createMutation.mutate(values); }
    });
  };

  return (
    <Modal title={role ? 'Edit Role' : 'Create Role'} open={open} onOk={onOk} onCancel={onClose} width={640}
      confirmLoading={createMutation.isPending || updateMutation.isPending}>
      <Form form={form} layout="vertical">
        <Form.Item name="name" label="Name" rules={[{ required: true }]}><Input disabled={!!role?.isSystem} /></Form.Item>
        <Form.Item name="description" label="Description"><Input.TextArea rows={2} /></Form.Item>
        <Form.Item name="permissionIds" label="Permissions">
          <Checkbox.Group style={{ width: '100%' }}>
            {Array.from(grouped.entries()).map(([resource, perms]) => (
              <div key={resource} style={{ marginBottom: 8 }}>
                <Text strong style={{ textTransform: 'capitalize' }}>{resource}</Text>
                <div style={{ marginLeft: 16 }}>
                  {perms.map(p => (
                    <Checkbox key={p.id} value={p.id}>{p.action}</Checkbox>
                  ))}
                </div>
              </div>
            ))}
          </Checkbox.Group>
        </Form.Item>
      </Form>
    </Modal>
  );
};
