import React from 'react';
import { usePermission } from '../hooks/usePermission';

interface Props {
  resource: string;
  action: string;
  children: React.ReactNode;
  fallback?: React.ReactNode;
}

export const PermissionGuard: React.FC<Props> = ({ resource, action, children, fallback = null }) => {
  const { hasPermission } = usePermission();
  return hasPermission(resource, action) ? <>{children}</> : <>{fallback}</>;
};
