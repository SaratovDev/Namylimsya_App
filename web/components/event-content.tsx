"use client";

import { useCallback, useEffect, useState } from "react";
import { notFound } from "next/navigation";
import { AlertCircle, RefreshCw } from "lucide-react";
import { EventDetail } from "./event-detail";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/lib/auth-context";
import { apiFetch } from "@/lib/api";

type ErrorKind = "not_found" | "forbidden" | "server" | null;

export function EventContent({ id }: { id: string }) {
  const { accessToken, isReady } = useAuth();
  const [event, setEvent] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);
  const [errorKind, setErrorKind] = useState<ErrorKind>(null);

  const loadEvent = useCallback(async () => {
    if (!isReady) return;
    setLoading(true);
    setErrorKind(null);
    try {
      const res = await apiFetch(`/events/${id}`, {}, accessToken);
      const data = await res.json();
      setEvent(data);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "";
      // 404 / 403 — семантические. Всё остальное — сервер/сеть.
      // apiFetch бросает Error с message из JSON или «HTTP <code>».
      if (msg.includes("not found") || msg.includes("404")) {
        setErrorKind("not_found");
      } else if (msg.includes("forbidden") || msg.includes("403")) {
        setErrorKind("forbidden");
      } else {
        setErrorKind("server");
      }
    } finally {
      setLoading(false);
    }
  }, [id, accessToken, isReady]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-on-mount: setState происходит после await (ложное срабатывание)
    void loadEvent();
  }, [loadEvent]);

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-12 w-3/4" />
        <Skeleton className="h-6 w-1/3" />
        <Skeleton className="h-64 w-full rounded-xl" />
      </div>
    );
  }

  if (errorKind === "not_found") {
    return notFound();
  }

  if (errorKind === "forbidden") {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <AlertCircle className="h-10 w-10 text-destructive mb-3" />
        <p className="text-sm text-muted-foreground mb-4">
          Это событие скрыто или недоступно.
        </p>
      </div>
    );
  }

  if (errorKind === "server" || !event) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <AlertCircle className="h-10 w-10 text-destructive mb-3" />
        <p className="text-sm text-muted-foreground mb-4">
          Не удалось загрузить событие. Проверьте соединение.
        </p>
        <Button variant="outline" onClick={() => void loadEvent()} className="gap-2">
          <RefreshCw className="h-4 w-4" />
          Повторить
        </Button>
      </div>
    );
  }

  return <EventDetail event={event as never} onRefresh={loadEvent} />;
}