import React, { useState } from 'react';
import { SectionBox } from '@kinvolk/headlamp-plugin/lib/CommonComponents';

interface Dashboard {
  label: string;
  uid: string;
}

const DASHBOARDS: Dashboard[] = [
  { label: 'vLLM', uid: 'vllm-overview' },
  { label: 'LiteLLM Gateway', uid: 'litellm-gateway' },
  { label: 'System', uid: 'system-overview' },
  { label: 'GPU', uid: 'gpu-overview' },
  { label: 'SLO', uid: 'slo-overview' },
  { label: 'Cost & Usage', uid: 'cost-usage' },
];

const GRAFANA_PORT = 30300;

function getGrafanaUrl(uid: string): string {
  const host = window.location.hostname;
  return `http://${host}:${GRAFANA_PORT}/d/${uid}/?orgId=1&kiosk`;
}

export default function MonitoringPage() {
  const [activeTab, setActiveTab] = useState(0);

  return (
    <SectionBox title="Monitoring" headerStyle="main">
      <div style={{ borderBottom: '1px solid #e0e0e0', display: 'flex', gap: '0' }}>
        {DASHBOARDS.map((d, i) => (
          <button
            key={d.uid}
            onClick={() => setActiveTab(i)}
            style={{
              padding: '10px 20px',
              border: 'none',
              borderBottom: activeTab === i ? '3px solid #1976d2' : '3px solid transparent',
              background: 'none',
              cursor: 'pointer',
              fontWeight: activeTab === i ? 600 : 400,
              color: activeTab === i ? '#1976d2' : '#666',
              fontSize: '14px',
              transition: 'all 0.2s',
            }}
          >
            {d.label}
          </button>
        ))}
      </div>
      <iframe
        src={getGrafanaUrl(DASHBOARDS[activeTab].uid)}
        style={{
          width: '100%',
          height: 'calc(100vh - 220px)',
          border: 'none',
          marginTop: '8px',
        }}
        title={DASHBOARDS[activeTab].label}
      />
    </SectionBox>
  );
}
