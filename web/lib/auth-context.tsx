"use client";

import {
  createContext,
  useContext,
  useState,
  useEffect,
  useRef,
  useCallback,
  ReactNode,
} from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { setRefreshHandler, setAuthFailHandler } from "./api";

const API_BASE =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";

type AuthContextType = {
  accessToken: string | null;
  setAccessToken: (token: string | null) => void;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refreshAccessToken: () => Promise<string | null>;
  isReady: boolean;
};

const AuthContext = createContext<AuthContextType>({
  accessToken: null,
  setAccessToken: () => {},
  login: async () => {},
  logout: async () => {},
  refreshAccessToken: async () => null,
  isReady: false,
});

export function AuthProvider({ children }: { children: ReactNode }) {
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [isReady, setIsReady] = useState(false);
  const router = useRouter();

  // Один in-flight refresh на всех. Параллельные 401 ждут один промис.
  const refreshPromiseRef = useRef<Promise<string | null> | null>(null);
  // Инкрементируется при logout — инвалидирует результат in-flight refresh.
  const logoutGenRef = useRef(0);

  const refreshAccessToken = useCallback(async (): Promise<string | null> => {
    if (refreshPromiseRef.current) return refreshPromiseRef.current;

    const gen = logoutGenRef.current;
    const promise = (async () => {
      try {
        const res = await fetch(`${API_BASE}/auth/refresh`, {
          method: "POST",
          credentials: "include",
        });
        if (!res.ok) return null;
        const data = await res.json();
        const token = data.access_token as string | undefined;
        if (!token) return null;
        // logout во время refresh — не воскрешаем сессию
        if (logoutGenRef.current !== gen) return null;
        setAccessToken(token);
        return token;
      } catch {
        return null;
      } finally {
        refreshPromiseRef.current = null;
      }
    })();

    refreshPromiseRef.current = promise;
    return promise;
  }, []);

  const login = useCallback(
    async (email: string, password: string) => {
      const res = await fetch(`${API_BASE}/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
        credentials: "include",
      });
      if (!res.ok) {
        const error = await res
          .json()
          .catch(() => ({ message: "Ошибка входа" }));
        throw new Error(error.message || "Неверный email или пароль");
      }
      const data = await res.json();
      logoutGenRef.current += 1; // сброс любых старых refresh
      setAccessToken(data.access_token);
      router.push("/");
      router.refresh();
    },
    [router]
  );

  const logout = useCallback(async () => {
    logoutGenRef.current += 1;
    refreshPromiseRef.current = null;
    try {
      await fetch(`${API_BASE}/auth/logout`, {
        method: "POST",
        credentials: "include",
      });
    } catch {
      // Бэкенд может быть недоступен — всё равно разлогиниваемся локально
    }
    setAccessToken(null);
    router.push("/login");
    router.refresh();
  }, [router]);

  const onAuthFail = useCallback(() => {
    toast.error("Сессия истекла, войдите заново");
    void logout();
  }, [logout]);

  // Восстановление сессии при монтировании. StrictMode вызовет дважды —
  // второй раз присоединится к тому же in-flight promise.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      await refreshAccessToken();
      if (!cancelled) setIsReady(true);
    })();
    return () => {
      cancelled = true;
    };
  }, [refreshAccessToken]);

  // Публикуем хендлеры для apiFetch. Порядок важен: этот effect объявлен
  // ДО restore-эффекта выше, чтобы refreshFn был установлен к моменту
  // первого запроса из дочерних компонентов.
  useEffect(() => {
    setRefreshHandler(refreshAccessToken);
    setAuthFailHandler(onAuthFail);
    return () => {
      setRefreshHandler(null);
      setAuthFailHandler(null);
    };
  }, [refreshAccessToken, onAuthFail]);

  return (
    <AuthContext.Provider
      value={{
        accessToken,
        setAccessToken,
        login,
        logout,
        refreshAccessToken,
        isReady,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export const useAuth = () => useContext(AuthContext);