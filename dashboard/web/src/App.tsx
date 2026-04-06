import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider, App as AntdApp } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useTheme } from './hooks/useTheme';
import { AppLayout } from './components/Layout/AppLayout';

import Overview from './pages/Overview/Overview';
import ModelList from './pages/Models/ModelList';
import ModelDetail from './pages/Models/ModelDetail';
import DeployWizard from './pages/Models/DeployWizard';
import FinetuneList from './pages/Finetune/FinetuneList';
import FinetuneDetail from './pages/Finetune/FinetuneDetail';
import CreateWizard from './pages/Finetune/CreateWizard';
import RagList from './pages/Rag/RagList';
import RagDetail from './pages/Rag/RagDetail';
import ServiceGrid from './pages/Services/ServiceGrid';
import ServiceEmbed from './pages/Services/ServiceEmbed';
import MonitoringDashboard from './pages/Monitoring/MonitoringDashboard';
import PlatformStatus from './pages/Platform/PlatformStatus';
import UserList from './pages/Users/UserList';
import RoleList from './pages/Users/RoleList';
import PermissionList from './pages/Users/PermissionList';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false },
  },
});

const Placeholder = ({ title }: { title: string }) => (
  <div><h2>{title}</h2><p>Coming soon...</p></div>
);

function ThemedApp() {
  const { algorithm } = useTheme();

  return (
    <ConfigProvider theme={{ algorithm, token: { colorPrimary: '#1677ff' } }}>
      <AntdApp>
        <BrowserRouter>
          <Routes>
            <Route element={<AppLayout />}>
              <Route path="/" element={<Navigate to="/overview" replace />} />
              <Route path="/overview" element={<Overview />} />
              <Route path="/models" element={<ModelList />} />
              <Route path="/models/deploy" element={<DeployWizard />} />
              <Route path="/models/:name" element={<ModelDetail />} />
              <Route path="/finetune" element={<FinetuneList />} />
              <Route path="/finetune/create" element={<CreateWizard />} />
              <Route path="/finetune/:name" element={<FinetuneDetail />} />
              <Route path="/rag" element={<RagList />} />
              <Route path="/rag/:id" element={<RagDetail />} />
              <Route path="/services" element={<ServiceGrid />} />
              <Route path="/services/:name" element={<ServiceEmbed />} />
              <Route path="/monitoring" element={<MonitoringDashboard />} />
              <Route path="/notebooks" element={<Placeholder title="Notebooks" />} />
              <Route path="/logs" element={<Placeholder title="Logs" />} />
              <Route path="/platform" element={<PlatformStatus />} />
              <Route path="/users" element={<UserList />} />
              <Route path="/users/roles" element={<RoleList />} />
              <Route path="/users/permissions" element={<PermissionList />} />
              <Route path="/profile" element={<Placeholder title="Profile" />} />
            </Route>
            <Route path="*" element={<Navigate to="/overview" replace />} />
          </Routes>
        </BrowserRouter>
      </AntdApp>
    </ConfigProvider>
  );
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemedApp />
    </QueryClientProvider>
  );
}

export default App;
