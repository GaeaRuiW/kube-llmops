import React from 'react';
import { Steps, Form, Input, Select, InputNumber, Button, Card, Space, Typography, message } from 'antd';
import { useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import apiClient from '../../api/client';

const { Title } = Typography;

const DeployWizard: React.FC = () => {
  const [current, setCurrent] = React.useState(0);
  const [form] = Form.useForm();
  const navigate = useNavigate();
  const qc = useQueryClient();

  const createMutation = useMutation({
    mutationFn: (values: Record<string, unknown>) => apiClient.post('/models', {
      metadata: { name: values.name },
      spec: {
        source: values.source,
        engine: values.engine || '',
        replicas: values.replicas ?? 1,
        resources: { gpu: values.gpu ?? 1, memory: values.memory || '16Gi', cpu: values.cpu || '4' },
      },
    }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['models'] }); message.success('Model deployed'); navigate('/models'); },
    onError: (e: Error) => message.error(e.message),
  });

  const steps = [
    { title: 'Model', content: (
      <>
        <Form.Item name="name" label="Name" rules={[{ required: true }]}><Input placeholder="my-model" /></Form.Item>
        <Form.Item name="source" label="HuggingFace Source" rules={[{ required: true }]}><Input placeholder="meta-llama/Llama-3.1-8B-Instruct" /></Form.Item>
        <Form.Item name="engine" label="Engine (auto-detect if empty)">
          <Select allowClear placeholder="Auto-detect" options={[
            { value: 'vllm', label: 'vLLM' }, { value: 'llamacpp', label: 'llama.cpp (GGUF)' },
            { value: 'tei', label: 'TEI (Embedding/Rerank)' },
          ]} />
        </Form.Item>
      </>
    )},
    { title: 'Resources', content: (
      <>
        <Form.Item name="replicas" label="Replicas" initialValue={1}><InputNumber min={1} max={10} /></Form.Item>
        <Form.Item name="gpu" label="GPUs" initialValue={1}><InputNumber min={0} max={8} /></Form.Item>
        <Form.Item name="memory" label="Memory" initialValue="16Gi"><Input /></Form.Item>
        <Form.Item name="cpu" label="CPU" initialValue="4"><Input /></Form.Item>
      </>
    )},
    { title: 'Confirm', content: (
      <Card>
        <pre>{JSON.stringify(form.getFieldsValue(), null, 2)}</pre>
      </Card>
    )},
  ];

  return (
    <div>
      <Title level={3}>Deploy Model</Title>
      <Steps current={current} items={steps.map(s => ({ title: s.title }))} style={{ marginBottom: 24 }} />
      <Form form={form} layout="vertical" style={{ maxWidth: 600 }}>
        {steps[current].content}
        <Space style={{ marginTop: 16 }}>
          {current > 0 && <Button onClick={() => setCurrent(c => c - 1)}>Previous</Button>}
          {current < steps.length - 1 && <Button type="primary" onClick={() => setCurrent(c => c + 1)}>Next</Button>}
          {current === steps.length - 1 && (
            <Button type="primary" loading={createMutation.isPending}
              onClick={() => form.validateFields().then(v => createMutation.mutate(v))}>
              Deploy
            </Button>
          )}
        </Space>
      </Form>
    </div>
  );
};

export default DeployWizard;
