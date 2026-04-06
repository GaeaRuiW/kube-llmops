import React from 'react';
import { Steps, Form, Input, Select, InputNumber, Button, Card, Space, Typography, message } from 'antd';
import { useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import apiClient from '../../api/client';

const { Title } = Typography;

const CreateWizard: React.FC = () => {
  const [current, setCurrent] = React.useState(0);
  const [form] = Form.useForm();
  const navigate = useNavigate();
  const qc = useQueryClient();

  const createMutation = useMutation({
    mutationFn: (values: Record<string, unknown>) => apiClient.post('/finetune', {
      metadata: { name: values.name },
      spec: {
        baseModel: values.baseModel,
        outputName: values.outputName,
        method: values.method,
        dataSource: { type: values.dataType, path: values.dataPath, format: values.dataFormat || 'alpaca' },
        training: { epochs: values.epochs ?? 3, batchSize: values.batchSize ?? 4, learningRate: values.learningRate || '2e-5' },
        resources: { gpu: values.gpu ?? 1, memory: values.memory || '32Gi', cpu: values.cpu || '8' },
      },
    }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['finetunes'] }); message.success('Created'); navigate('/finetune'); },
    onError: (e: Error) => message.error(e.message),
  });

  const steps = [
    { title: 'Model', content: (
      <>
        <Form.Item name="name" label="Name" rules={[{ required: true }]}><Input placeholder="my-finetune" /></Form.Item>
        <Form.Item name="baseModel" label="Base Model" rules={[{ required: true }]}><Input placeholder="meta-llama/Llama-3.1-8B-Instruct" /></Form.Item>
        <Form.Item name="outputName" label="Output Name" rules={[{ required: true }]}><Input placeholder="my-model-finetuned" /></Form.Item>
        <Form.Item name="method" label="Method" initialValue="lora">
          <Select options={[{ value: 'lora', label: 'LoRA' }, { value: 'qlora', label: 'QLoRA' }, { value: 'full', label: 'Full' }]} />
        </Form.Item>
      </>
    )},
    { title: 'Data', content: (
      <>
        <Form.Item name="dataType" label="Source Type" initialValue="minio">
          <Select options={[{ value: 'minio', label: 'MinIO (S3)' }, { value: 'huggingface', label: 'HuggingFace' }, { value: 'pvc', label: 'PVC' }]} />
        </Form.Item>
        <Form.Item name="dataPath" label="Path" rules={[{ required: true }]}><Input placeholder="s3://finetune-data/train.json" /></Form.Item>
        <Form.Item name="dataFormat" label="Format" initialValue="alpaca">
          <Select options={[{ value: 'alpaca', label: 'Alpaca' }, { value: 'sharegpt', label: 'ShareGPT' }, { value: 'custom', label: 'Custom' }]} />
        </Form.Item>
      </>
    )},
    { title: 'Training', content: (
      <>
        <Form.Item name="epochs" label="Epochs" initialValue={3}><InputNumber min={1} max={100} /></Form.Item>
        <Form.Item name="batchSize" label="Batch Size" initialValue={4}><InputNumber min={1} max={64} /></Form.Item>
        <Form.Item name="learningRate" label="Learning Rate" initialValue="2e-5"><Input /></Form.Item>
        <Form.Item name="gpu" label="GPUs" initialValue={1}><InputNumber min={1} max={8} /></Form.Item>
        <Form.Item name="memory" label="Memory" initialValue="32Gi"><Input /></Form.Item>
      </>
    )},
    { title: 'Confirm', content: <Card><pre>{JSON.stringify(form.getFieldsValue(), null, 2)}</pre></Card> },
  ];

  return (
    <div>
      <Title level={3}>Create Fine-tune Job</Title>
      <Steps current={current} items={steps.map(s => ({ title: s.title }))} style={{ marginBottom: 24 }} />
      <Form form={form} layout="vertical" style={{ maxWidth: 600 }}>
        {steps[current].content}
        <Space style={{ marginTop: 16 }}>
          {current > 0 && <Button onClick={() => setCurrent(c => c - 1)}>Previous</Button>}
          {current < steps.length - 1 && <Button type="primary" onClick={() => setCurrent(c => c + 1)}>Next</Button>}
          {current === steps.length - 1 && (
            <Button type="primary" loading={createMutation.isPending}
              onClick={() => form.validateFields().then(v => createMutation.mutate(v))}>
              Create
            </Button>
          )}
        </Space>
      </Form>
    </div>
  );
};

export default CreateWizard;
