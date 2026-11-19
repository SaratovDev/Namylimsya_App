"use client";

import { useState } from "react";
import { toast } from "sonner";
import { apiFetch } from "./api";
import { useAuth } from "./auth-context";

export function useEventActions(
  eventId: string,
  applicationId?: string | null,
  onRefresh?: () => void
) {
  const { accessToken } = useAuth();
  const [loading, setLoading] = useState(false);

  const apply = async () => {
    if (!accessToken) {
      toast.error("Войдите, чтобы подать заявку");
      return;
    }
    setLoading(true);
    try {
      await apiFetch(`/events/${eventId}/applications`, { method: "POST" }, accessToken);
      toast.success("Заявка отправлена!");
      onRefresh?.();
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "Не удалось подать заявку";
      toast.error(message);
    } finally {
      setLoading(false);
    }
  };

  const cancelApplication = async () => {
    if (!applicationId) {
      toast.error("Не найдена активная заявка");
      return;
    }
    if (!accessToken) {
      toast.error("Сессия истекла, войдите заново");
      return;
    }
    setLoading(true);
    try {
      await apiFetch(
        `/applications/${applicationId}/cancel`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ reason: "Планы изменились" }),
        },
        accessToken
      );
      toast.success("Заявка отменена");
      onRefresh?.();
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "Не удалось отменить заявку";
      toast.error(message);
    } finally {
      setLoading(false);
    }
  };

  const share = () => {
    navigator.clipboard.writeText(`${window.location.origin}/events/${eventId}`);
    toast.success("Ссылка скопирована");
  };

  return { apply, cancelApplication, share, loading };
}