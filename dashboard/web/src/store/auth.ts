import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface User {
  id: string;
  email: string;
  displayName: string;
  avatar?: string;
}

interface Permission {
  resource: string;
  action: string;
}

type Theme = 'light' | 'dark' | 'auto';

interface AuthState {
  token: string | null;
  user: User | null;
  permissions: Permission[];
  theme: Theme;
  sidebarCollapsed: boolean;
  namespace: string;

  setToken: (token: string | null) => void;
  setUser: (user: User | null) => void;
  setPermissions: (permissions: Permission[]) => void;
  setTheme: (theme: Theme) => void;
  setNamespace: (ns: string) => void;
  toggleSidebar: () => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      permissions: [],
      theme: 'auto' as Theme,
      sidebarCollapsed: false,
      namespace: 'default',

      setToken: (token) => set({ token }),
      setUser: (user) => set({ user }),
      setPermissions: (permissions) => set({ permissions }),
      setTheme: (theme) => set({ theme }),
      setNamespace: (ns) => set({ namespace: ns }),
      toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
      logout: () => set({ token: null, user: null, permissions: [] }),
    }),
    {
      name: 'dashboard-store',
      partialize: (state) => ({
        theme: state.theme,
        sidebarCollapsed: state.sidebarCollapsed,
        namespace: state.namespace,
        token: state.token,
      }),
    }
  )
);
