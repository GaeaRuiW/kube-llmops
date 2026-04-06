import React from 'react';
import { Modal, Form, Input, message } from 'antd';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import apiClient from '../../api/client';

interface Props {
  open: boolean;
  onClose: () => void;
}

export const PermissionForm: React.FC<Props> = ({ open, onClose }) => {
  const [form] = Form.useForm();
  const qc = useQueryClient();

  const createMutation = useMutation({
    mutationFn: (values: Record<string, unknown>) => apiClient.post('/permissions', values),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['permissions'] }); message.success('Created'); form.resetFields(); onClose(); },
  });

  return (
    <Modal title="Create Permission" open={open}
      onOk={() => form.validateFields().then(v => createMutation.mutate(v))}
      onCancel={onClose} confirmLoading={createMutation.isPending}>
      <Form form={form} layout="vertical">
        <Form.Item name="resource" label="Resource" rules={[{ required: true }]}><Input placeholder="e.g. models" /></Form.Item>
        <Form.Item name="action" label="Action" rules={[{ required: true }]}><Input placeholder="e.g. view" /></Form.Item>
        <Form.Item name="description" label="Description"><Input.TextArea /></Form.Item>
      </Form>
    </Modal>
  );
};
