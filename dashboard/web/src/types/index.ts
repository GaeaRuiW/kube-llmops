// API response types
export interface ApiResponse<T> {
  data: T;
  message?: string;
}

// Auth types
export interface AuthUser {
  id: string;
  email: string;
  displayName: string;
  avatar?: string;
}

export interface AuthPermission {
  resource: string;
  action: string;
}

// Model types
export interface ModelDeployment {
  metadata: { name: string; namespace: string; creationTimestamp: string };
  spec: {
    source: string;
    engine: string;
    replicas?: number;
    resources: { gpu: number; memory: string; cpu: string };
    engineArgs?: Record<string, string>;
    canary?: { source: string; weight: number };
  };
  status: {
    phase: string;
    engine: string;
    endpoint: string;
    readyReplicas: number;
    totalReplicas: number;
    conditions?: Array<{ type: string; status: string; message: string }>;
    canary?: { phase: string; endpoint: string; readyReplicas: number };
  };
}

// FineTuneRun types
export interface FineTuneRun {
  metadata: { name: string; namespace: string; creationTimestamp: string };
  spec: {
    baseModel: string;
    outputName: string;
    method: string;
    dataSource: { type: string; path: string; format: string };
    training: { epochs: number; batchSize: number; learningRate: string };
    resources: { gpu: number; memory: string; cpu: string };
  };
  status: {
    phase: string;
    argoWorkflow: string;
    startTime?: string;
    completionTime?: string;
    metrics: { trainLoss: string; evalLoss: string; trainingDuration: string };
    mlflow: { runId: string; experimentName: string };
    qualityGate: { passed: boolean; message: string };
    outputModel: { source: string; modelDeployment: string };
  };
}

// Service types
export interface ServiceInfo {
  name: string;
  description: string;
  icon: string;
  phase: string;
  endpoint?: string;
  proxyPath: string;
}

// RBAC types
export interface User {
  id: string;
  keycloakId: string;
  email: string;
  displayName: string;
  avatar?: string;
  enabled: boolean;
  lastLogin?: string;
  roles?: Role[];
}

export interface Role {
  id: string;
  name: string;
  description: string;
  isSystem: boolean;
  permissions?: Permission[];
}

export interface Permission {
  id: string;
  resource: string;
  action: string;
  description: string;
  isSystem: boolean;
}
