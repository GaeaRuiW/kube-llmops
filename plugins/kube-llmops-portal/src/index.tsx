import {
  registerRoute,
  registerSidebarEntry,
} from '@kinvolk/headlamp-plugin/lib';
import ServiceLinksPage from './ServiceLinksPage';
import MonitoringPage from './MonitoringPage';

// ---- Sidebar section ----
// Parent entry needs a url so clicking it navigates (and auto-expands children).
// Without a url, Headlamp v0.25.x renders it as <a href="/"> which just reloads root.
registerSidebarEntry({
  parent: null,
  name: 'llmops',
  label: 'LLMOps',
  url: '/kube-llmops/services',
  icon: 'mdi:brain',
});

registerSidebarEntry({
  parent: 'llmops',
  name: 'llmops-services',
  label: 'Service Links',
  url: '/kube-llmops/services',
  icon: 'mdi:link-variant',
});

registerSidebarEntry({
  parent: 'llmops',
  name: 'llmops-monitoring',
  label: 'Monitoring',
  url: '/kube-llmops/monitoring',
  icon: 'mdi:chart-line',
});

// ---- Routes ----
registerRoute({
  path: '/kube-llmops/services',
  sidebar: 'llmops-services',
  name: 'llmops-services',
  exact: true,
  component: () => <ServiceLinksPage />,
});

registerRoute({
  path: '/kube-llmops/monitoring',
  sidebar: 'llmops-monitoring',
  name: 'llmops-monitoring',
  exact: true,
  component: () => <MonitoringPage />,
});
