import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider, theme as antdTheme, App as AntdApp } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

// Placeholder pages - will be replaced in later tasks
const Placeholder = ({ title }: { title: string }) => (
  <div style={{ padding: 24 }}>
    <h2>{title}</h2>
    <p>Coming soon...</p>
  </div>
);

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ConfigProvider
        theme={{
          algorithm: antdTheme.defaultAlgorithm,
          token: {
            colorPrimary: '#1677ff',
          },
        }}
      >
        <AntdApp>
          <BrowserRouter>
            <Routes>
              <Route path="/" element={<Navigate to="/overview" replace />} />
              <Route path="/overview" element={<Placeholder title="Overview" />} />
              <Route path="/models" element={<Placeholder title="Models" />} />
              <Route path="/models/:name" element={<Placeholder title="Model Detail" />} />
              <Route path="/models/deploy" element={<Placeholder title="Deploy Model" />} />
              <Route path="/finetune" element={<Placeholder title="Fine-tuning" />} />
              <Route path="/finetune/:name" element={<Placeholder title="Fine-tune Detail" />} />
              <Route path="/finetune/create" element={<Placeholder title="Create Fine-tune" />} />
              <Route path="/rag" element={<Placeholder title="RAG Knowledge Bases" />} />
              <Route path="/rag/:id" element={<Placeholder title="Knowledge Base Detail" />} />
              <Route path="/services" element={<Placeholder title="Services" />} />
              <Route path="/services/:name" element={<Placeholder title="Service" />} />
              <Route path="/monitoring" element={<Placeholder title="Monitoring" />} />
              <Route path="/notebooks" element={<Placeholder title="Notebooks" />} />
              <Route path="/logs" element={<Placeholder title="Logs" />} />
              <Route path="/platform" element={<Placeholder title="Platform" />} />
              <Route path="/users" element={<Placeholder title="Users" />} />
              <Route path="/users/roles" element={<Placeholder title="Roles" />} />
              <Route path="/users/permissions" element={<Placeholder title="Permissions" />} />
              <Route path="/profile" element={<Placeholder title="Profile" />} />
              <Route path="*" element={<Navigate to="/overview" replace />} />
            </Routes>
          </BrowserRouter>
        </AntdApp>
      </ConfigProvider>
    </QueryClientProvider>
  );
}

export default App;
