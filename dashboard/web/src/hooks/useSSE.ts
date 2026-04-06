import { useEffect, useRef } from 'react';
import { useAuthStore } from '../store/auth';

interface SSEEvent {
  type: string;
  object: unknown;
}

export function useSSE(onMessage: (evt: SSEEvent) => void) {
  const token = useAuthStore((s) => s.token);
  const cbRef = useRef(onMessage);
  cbRef.current = onMessage;

  useEffect(() => {
    if (!token) return;

    let es: EventSource | null = null;
    let timer: ReturnType<typeof setTimeout> | null = null;

    const connect = () => {
      es = new EventSource(`/api/v1/events?token=${encodeURIComponent(token)}`);

      es.onmessage = (e) => {
        try {
          cbRef.current(JSON.parse(e.data));
        } catch { /* ignore parse errors */ }
      };

      es.onerror = () => {
        es?.close();
        timer = setTimeout(connect, 5000);
      };
    };

    connect();

    return () => {
      es?.close();
      if (timer) clearTimeout(timer);
    };
  }, [token]);
}
