"use client";

import { useState, useEffect, useRef } from "react";
import { useAuth } from "./auth-context";
import { apiFetch } from "./api";
import { isTokenExpiringSoon } from "./jwt";

export type ChatMessage = {
  id: string;
  event_id: string;
  user_id: string;
  text: string;
  reply_to_id: string | null;
  created_at: string;
  user_name: string;
  user_avatar: string | null;
};

const WS_URL = process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8080/ws";

// Close-коды (синхронизированы с api/internal/handlers/ws.go и ws/hub.go):
//   4401 — нет/протух токен → refresh;
//   4403 — нет доступа / бан → denied;
//   4408 — соединение заменено более новым (per-user cap 3, LRU) →
//          не reconnect, показать юзеру.
const WS_CLOSE_UNAUTHORIZED = 4401;
const WS_CLOSE_FORBIDDEN = 4403;
const WS_CLOSE_REPLACED = 4408;

export function useChat(eventId: string, token: string | null) {
  const { refreshAccessToken } = useAuth();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [loading, setLoading] = useState(token === null);
  const [connected, setConnected] = useState(false);
  const [denied, setDenied] = useState(false);
  const [lastError, setLastError] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const tokenRef = useRef(token);

  // Обновляем ref в эффекте, а не во время render — react-hooks/refs
  // требует этого. Для WS-цикла синхронность не нужна: connect() читает
  // tokenRef.current уже после commit'а.
  useEffect(() => {
    tokenRef.current = token;
  }, [token]);

  useEffect(() => {
    if (!token) return;

    let stopped = false;
    let retry = 0;
    let timer: ReturnType<typeof setTimeout> | null = null;

    const fetchHistory = async () => {
      try {
        const res = await apiFetch(
          `/events/${eventId}/messages`,
          {},
          tokenRef.current
        );
        const data = await res.json();
        setMessages(data.messages || []);
      } catch {
        // 403 — доступа нет
      } finally {
        setLoading(false);
      }
    };

    const connect = async () => {
      if (stopped) return;

      // Превентивный refresh: браузер на 403 при WS-upgrade даёт 1006,
      // отличить «протух токен» от «нет доступа» нельзя. Решаем заранее.
      let currentToken = tokenRef.current;
      if (currentToken && isTokenExpiringSoon(currentToken, 60)) {
        const newToken = await refreshAccessToken();
        if (stopped) return;
        if (newToken) {
          currentToken = newToken;
          tokenRef.current = newToken;
        }
      }
      if (!currentToken) {
        setLastError("Сессия истекла. Войдите заново.");
        return;
      }

      // Токен в subprotocol (Sec-WebSocket-Protocol), НЕ в URL — чтобы
      // access-токен не утекал в access-логи reverse-proxy и историю браузера.
      // См. api/internal/handlers/ws.go: hasBearerSubprotocol.
      const ws = new WebSocket(
        `${WS_URL}/events/${eventId}`,
        ["bearer", currentToken]
      );
      wsRef.current = ws;

      ws.onopen = () => {
        retry = 0;
        setConnected(true);
        setDenied(false);
        setLastError(null);
        void fetchHistory();
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          if (data.type === "message") {
            setMessages((prev) => {
              if (prev.some((m) => m.id === data.message.id)) return prev;
              return [...prev, data.message];
            });
          } else if (data.error) {
            setLastError(String(data.error));
          }
        } catch (e) {
          console.error("WS parse error", e);
        }
      };

      ws.onclose = async (ev) => {
        // Чужой onclose (от предыдущего WS) не должен затирать ссылку
        if (wsRef.current === ws) {
          wsRef.current = null;
          setConnected(false);
        }
        if (stopped) return;

        // LRU per-user cap: сервер заменил наше соединение более новым
        // (4408). Это не ошибка и не «нет доступа» — просто открыто в
        // другом окне. Не реконнектимся, иначе будет вечная гонка двух
        // вкладок. Юзер увидит сообщение, пока не закроет другое окно.
        if (ev.code === WS_CLOSE_REPLACED) {
          setLastError("Соединение открыто в другом окне. Закройте его и обновите страницу.");
          return;
        }

        // Токен протух — refresh и один ретрай.
        if (ev.code === WS_CLOSE_UNAUTHORIZED) {
          const newToken = await refreshAccessToken();
          if (stopped) return;
          if (newToken) {
            tokenRef.current = newToken;
            timer = setTimeout(connect, 500);
            return;
          }
          setDenied(true);
          return;
        }

        // Clean close (сервер штатно закрыл или вкладка уходит в фон).
        if (ev.code === 1000 || ev.code === 1001) return;

        // Нет доступа к чату / забанен.
        if (ev.code === WS_CLOSE_FORBIDDEN) {
          setDenied(true);
          return;
        }

        // 1006 / 1008 / всё остальное — transient, backoff.
        if (retry >= 10) {
          setLastError(
            "Не удалось восстановить соединение. Перезагрузите страницу."
          );
          return;
        }
        const delay = Math.min(30000, 1000 * 2 ** retry);
        retry += 1;
        timer = setTimeout(connect, delay);
      };

      ws.onerror = () => {
        // onclose придёт сам
      };
    };

    void connect();

    // После sleep/visibility браузер не всегда эмитит onclose.
    // При возврате: живой WS не трогаем, мёртвый — reconnect.
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
  }, [eventId, token, refreshAccessToken]);

  const sendMessage = (text: string): boolean => {
    if (!text.trim()) return false;
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      setLastError("Соединение потеряно, переподключаемся…");
      return false;
    }
    if (ws.bufferedAmount > 64 * 1024) {
      setLastError("Соединение потеряно, переподключаемся…");
      ws.close();
      return false;
    }
    setLastError(null);
    ws.send(JSON.stringify({ text: text.trim() }));
    return true;
  };

  return { messages, loading, connected, denied, lastError, sendMessage };
}