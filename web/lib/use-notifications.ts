"use client";

import { useState, useEffect, useRef } from "react";
import { useAuth } from "./auth-context";
import { apiFetch } from "./api";
import { isTokenExpiringSoon } from "./jwt";

export type NotificationPayload = {
  event_id?: string;
  event_title?: string;
  application_id?: string;
  applicant_id?: string;
  reason?: string;
  [key: string]: unknown;
};

export type Notification = {
  id: string;
  type: string;
  payload: NotificationPayload;
  is_read: boolean;
  created_at: string;
};

const WS_URL = process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8080/ws";

// См. use-chat.ts.
const WS_CLOSE_UNAUTHORIZED = 4401;
const WS_CLOSE_FORBIDDEN = 4403;
const WS_CLOSE_REPLACED = 4408;

export function useNotifications(token: string | null) {
  const { refreshAccessToken } = useAuth();
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loading, setLoading] = useState(token !== null);
  const wsRef = useRef<WebSocket | null>(null);
  const tokenRef = useRef(token);

  // См. use-chat.ts.
  useEffect(() => {
    tokenRef.current = token;
  }, [token]);

  useEffect(() => {
    if (!token) return;

    let stopped = false;
    let retry = 0;
    let timer: ReturnType<typeof setTimeout> | null = null;

    const fetchNotifications = async () => {
      try {
        const res = await apiFetch(`/notifications`, {}, tokenRef.current);
        const data = await res.json();
        setNotifications(data.notifications || []);
        setUnreadCount(data.unread || 0);
      } catch {
        // ignore
      } finally {
        setLoading(false);
      }
    };

    const connect = async () => {
      if (stopped) return;

      let currentToken = tokenRef.current;
      if (currentToken && isTokenExpiringSoon(currentToken, 60)) {
        const newToken = await refreshAccessToken();
        if (stopped) return;
        if (newToken) {
          currentToken = newToken;
          tokenRef.current = newToken;
        }
      }
      if (!currentToken) return;

      // Токен в subprotocol (см. use-chat.ts и ws.go).
      const ws = new WebSocket(
        `${WS_URL}/notifications`,
        ["bearer", currentToken]
      );
      wsRef.current = ws;

      ws.onopen = () => {
        retry = 0;
        void fetchNotifications();
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          if (data.type === "notification") {
            setNotifications((prev) => {
              if (prev.some((n) => n.id === data.data.id)) return prev;
              return [data.data, ...prev];
            });
            setUnreadCount((prev) => prev + 1);
          }
        } catch (e) {
          console.error("WS parse error", e);
        }
      };

      ws.onclose = async (ev) => {
        if (wsRef.current === ws) wsRef.current = null;
        if (stopped) return;

        // LRU per-user cap: не реконнектимся (см. use-chat.ts).
        if (ev.code === WS_CLOSE_REPLACED) {
          return;
        }

        if (ev.code === WS_CLOSE_UNAUTHORIZED) {
          const newToken = await refreshAccessToken();
          if (stopped) return;
          if (newToken) {
            tokenRef.current = newToken;
            timer = setTimeout(connect, 500);
            return;
          }
          return;
        }

        if (ev.code === 1000 || ev.code === 1001) return;
        if (ev.code === WS_CLOSE_FORBIDDEN) return;

        if (retry >= 10) return;
        const delay = Math.min(30000, 1000 * 2 ** retry);
        retry += 1;
        timer = setTimeout(connect, delay);
      };

      ws.onerror = () => {};
    };

    void connect();

    const onVisibility = () => {
      if (document.visibilityState !== "visible") return;
      const ws = wsRef.current;
      if (
        ws &&
        (ws.readyState === WebSocket.OPEN ||
          ws.readyState === WebSocket.CONNECTING)
      ) {
        return;
      }
      void connect();
    };
    document.addEventListener("visibilitychange", onVisibility);

    return () => {
      document.removeEventListener("visibilitychange", onVisibility);
      stopped = true;
      if (timer) clearTimeout(timer);
      const ws = wsRef.current;
      if (ws && ws === wsRef.current) wsRef.current = null;
      ws?.close();
    };
  }, [token, refreshAccessToken]);

  const markAsRead = async (id: string) => {
    if (!token) return;
    try {
      await apiFetch(`/notifications/${id}/read`, { method: "PATCH" }, token);
      setNotifications((prev) =>
        prev.map((n) => (n.id === id ? { ...n, is_read: true } : n))
      );
      setUnreadCount((prev) => Math.max(0, prev - 1));
    } catch {}
  };

  const markAllAsRead = async () => {
    if (!token) return;
    try {
      await apiFetch(`/notifications/read-all`, { method: "PATCH" }, token);
      setNotifications((prev) => prev.map((n) => ({ ...n, is_read: true })));
      setUnreadCount(0);
    } catch {}
  };

  return { notifications, unreadCount, loading, markAsRead, markAllAsRead };
}