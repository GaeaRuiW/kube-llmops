import { useAuthStore } from '../store/auth';

export function usePermission() {
  const token = useAuthStore((s) => s.token);
  const permissions = useAuthStore((s) => s.permissions);

  // Dev mode: grant all permissions when no OIDC token is present
  const devMode = !token;

  const hasPermission = (resource: string, action: string): boolean =>
    devMode || permissions.some((p) => p.resource === resource && p.action === action);

  const hasAnyPermission = (checks: [string, string][]): boolean =>
    devMode || checks.some(([r, a]) => hasPermission(r, a));

  const hasAllPermissions = (checks: [string, string][]): boolean =>
    devMode || checks.every(([r, a]) => hasPermission(r, a));

  return { hasPermission, hasAnyPermission, hasAllPermissions };
}
