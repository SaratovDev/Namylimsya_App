"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { useSearchParams } from "next/navigation";
import { AlertCircle, RefreshCw } from "lucide-react";
import { EventCard } from "./event-card";
import { Event } from "@/lib/types";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

type EventFeedProps = {
  /** Endpoint API. По умолчанию "/events". Для архива — "/events/archive". */
  endpoint?: string;
  /** Текст при пустом списке. */
  emptyText?: string;
};

export function EventFeed({
  endpoint = "/events",
  emptyText = "Пока нет событий с такими фильтрами.",
}: EventFeedProps = {}) {
  const { accessToken } = useAuth();
  const searchParams = useSearchParams();

  const [events, setEvents] = useState<Event[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [hasMore, setHasMore] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const observerTarget = useRef<HTMLDivElement>(null);

  const categories = searchParams.get("categories") || "";
  const city = searchParams.get("city") || "";
  const dateFrom = searchParams.get("date_from") || "";
  const dateTo = searchParams.get("date_to") || "";

  const fetchEvents = useCallback(
    async (currentCursor: string | null, reset: boolean) => {
      setLoading(true);
      setLoadError(null);
      try {
        const params = new URLSearchParams();
        if (categories) params.set("category", categories);
        if (city) params.set("city", city);
        if (dateFrom) params.set("date_from", dateFrom);
        if (dateTo) params.set("date_to", dateTo);
        if (currentCursor) params.set("cursor", currentCursor);
        params.set("limit", "20");

        const res = await apiFetch(`${endpoint}?${params.toString()}`, {}, accessToken);        const data = (await res.json()) as { events: Event[]; next_cursor: string | null };

        setEvents((prev) => (reset ? data.events : [...prev, ...data.events]));
        setCursor(data.next_cursor);
        setHasMore(!!data.next_cursor);
      } catch (err) {
        // Не маскируем ошибку пустым списком: показываем error-state
        // с retry. Иначе юзер видит «Пока нет событий» при упавшем API.
        const msg = err instanceof Error ? err.message : "Не удалось загрузить события";
        setLoadError(msg);
        if (reset) setEvents([]);
      } finally {
        setLoading(false);
      }
    },
    [categories, city, accessToken, dateFrom, dateTo, endpoint]
  );

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchEvents(null, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [categories, city, accessToken, dateFrom, dateTo]);

  const loadMore = useCallback(() => {
    if (loading || !hasMore || !cursor || loadError) return;
    fetchEvents(cursor, false);
  }, [loading, hasMore, cursor, loadError, fetchEvents]);

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) loadMore();
      },
      { threshold: 0.1, rootMargin: "200px" }
    );
    if (observerTarget.current) observer.observe(observerTarget.current);
    return () => observer.disconnect();
  }, [loadMore]);

  if (loading && events.length === 0) {
    return (
      <div className="space-y-4">
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-56 w-full rounded-xl" />
        ))}
      </div>
    );
  }

  // Первая загрузка упала — error-state с retry.
  if (loadError && events.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <AlertCircle className="h-10 w-10 text-destructive mb-3" />
        <p className="text-sm text-muted-foreground mb-4">{loadError}</p>
        <Button
          variant="outline"
          onClick={() => void fetchEvents(null, true)}
          className="gap-2"
        >
          <RefreshCw className="h-4 w-4" />
          Повторить
        </Button>
      </div>
    );
  }

  // Честное «нет событий» — API ответил 200, но список пуст.
  if (events.length === 0) {
    return (
      <div className="text-center text-muted-foreground py-12">
        {emptyText}
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {events.map((e) => (
        <EventCard
          key={e.id}
          event={e}
          onRefresh={() => fetchEvents(null, true)}
        />
      ))}

      {hasMore && (
        <div ref={observerTarget} className="py-6 flex justify-center">
          {loading && <Skeleton className="h-56 w-full rounded-xl" />}
        </div>
      )}

      {/* Догрузка упала — показываем inline-ошибку под уже загруженными карточками. */}
      {loadError && events.length > 0 && (
        <div className="flex flex-col items-center py-4 text-center">
          <p className="text-sm text-destructive mb-2">{loadError}</p>
          <Button
            variant="outline"
            size="sm"
            onClick={() => void fetchEvents(cursor, false)}
            className="gap-2"
          >
            <RefreshCw className="h-4 w-4" />
            Повторить
          </Button>
        </div>
      )}

      {!hasMore && !loadError && events.length > 0 && (
        <p className="text-center text-sm text-muted-foreground py-6">
          Вы посмотрели все события.
        </p>
      )}
    </div>
  );
}