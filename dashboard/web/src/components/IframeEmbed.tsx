import React from 'react';
import { Spin } from 'antd';
import { useTheme } from '../hooks/useTheme';

interface Props {
  src: string;
  title?: string;
}

export const IframeEmbed: React.FC<Props> = ({ src, title }) => {
  const [loading, setLoading] = React.useState(true);
  const { isDark } = useTheme();

  const finalSrc = src.includes('grafana') ? `${src}${src.includes('?') ? '&' : '?'}theme=${isDark ? 'dark' : 'light'}` : src;

  return (
    <div style={{ position: 'relative', width: '100%', height: 'calc(100vh - 180px)' }}>
      {loading && <div style={{ position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%,-50%)' }}><Spin size="large" /></div>}
      <iframe
        src={finalSrc}
        title={title || 'Embedded Service'}
        style={{ width: '100%', height: '100%', border: 'none' }}
        onLoad={() => setLoading(false)}
      />
    </div>
  );
};
