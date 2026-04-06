import { useEffect, useMemo } from 'react';
import { theme as antdTheme } from 'antd';
import { useAuthStore } from '../store/auth';

export function useTheme() {
  const themeMode = useAuthStore((s) => s.theme);
  const setTheme = useAuthStore((s) => s.setTheme);

  const isDark = useMemo(() => {
    if (themeMode === 'auto') {
      return window.matchMedia('(prefers-color-scheme: dark)').matches;
    }
    return themeMode === 'dark';
  }, [themeMode]);

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', isDark ? 'dark' : 'light');
  }, [isDark]);

  const algorithm = isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm;

  const cycleTheme = () => {
    const order: Array<'light' | 'dark' | 'auto'> = ['light', 'dark', 'auto'];
    const idx = order.indexOf(themeMode);
    setTheme(order[(idx + 1) % order.length]);
  };

  return { isDark, algorithm, themeMode, cycleTheme };
}
