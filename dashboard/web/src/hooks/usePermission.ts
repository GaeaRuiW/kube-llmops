import { useAuthStore } from '../store/auth';

export function usePermission() {
  const permissions = useAuthStore((s) => s.permissions);

  const hasPermission = (resource: string, action: string): boolean =>
    permissions.some((p) => p.resource === resource && p.action === action);

  const hasAnyPermission = (checks: [string, string][]): boolean =>
    checks.some(([r, a]) => hasPermission(r, a));

  const hasAllPermissions = (checks: [string, string][]): boolean =>
    checks.every(([r, a]) => hasPermission(r, a));

  return { hasPermission, hasAnyPermission, hasAllPermissions };
}
