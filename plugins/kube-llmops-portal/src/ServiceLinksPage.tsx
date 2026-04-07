import React from 'react';
import { SectionBox } from '@kinvolk/headlamp-plugin/lib/CommonComponents';

interface ServiceLink {
  name: string;
  port: number;
  description: string;
  icon: string;
  color: string;
}

const SERVICES: ServiceLink[] = [
  { name: 'Grafana', port: 30300, description: 'Monitoring dashboards', icon: '📊', color: '#FF6B35' },
  { name: 'Langfuse', port: 30301, description: 'LLM tracing & analytics', icon: '🔍', color: '#6366F1' },
  { name: 'LiteLLM', port: 30400, description: 'AI gateway', icon: '🤖', color: '#10B981' },
  { name: 'Dify', port: 30500, description: 'RAG platform', icon: '💬', color: '#3B82F6' },
  { name: 'Keycloak', port: 30808, description: 'SSO & identity', icon: '🔐', color: '#EF4444' },
  { name: 'Prometheus', port: 30909, description: 'Metrics', icon: '📈', color: '#F59E0B' },
  { name: 'MinIO', port: 30901, description: 'Object storage console', icon: '🗄️', color: '#C4032B' },
  { name: 'MLflow', port: 30505, description: 'Experiment tracking', icon: '🧪', color: '#0194E2' },
  { name: 'JupyterHub', port: 30888, description: 'Notebooks', icon: '📓', color: '#F37626' },
];

function getBaseUrl(): string {
  return window.location.hostname;
}

export default function ServiceLinksPage() {
  const host = getBaseUrl();

  return (
    <SectionBox title="Service Links" headerStyle="main">
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
          gap: '16px',
          padding: '16px',
        }}
      >
        {SERVICES.map((svc) => (
          <div
            key={svc.name}
            style={{
              border: '1px solid #e0e0e0',
              borderRadius: '8px',
              padding: '20px',
              cursor: 'pointer',
              transition: 'box-shadow 0.2s, transform 0.2s',
              backgroundColor: '#fff',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.boxShadow = '0 4px 12px rgba(0,0,0,0.15)';
              e.currentTarget.style.transform = 'translateY(-2px)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.boxShadow = 'none';
              e.currentTarget.style.transform = 'none';
            }}
            onClick={() => window.open(`http://${host}:${svc.port}`, '_blank')}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '8px' }}>
              <span style={{ fontSize: '28px' }}>{svc.icon}</span>
              <div>
                <div style={{ fontWeight: 600, fontSize: '16px' }}>{svc.name}</div>
                <div style={{ color: '#666', fontSize: '13px' }}>{svc.description}</div>
              </div>
            </div>
            <div style={{ fontSize: '12px', color: '#999', marginTop: '8px' }}>
              {host}:{svc.port}
            </div>
          </div>
        ))}
      </div>
    </SectionBox>
  );
}
