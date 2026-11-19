"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import {
  Search,
  AlertCircle,
  Loader2,
  ChevronLeft,
  ChevronRight,
  EyeOff,
  Ban,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { AdminGuard } from "@/components/admin-guard";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { AdminEventListItem } from "@/lib/types";
import { categoryLabel } from "@/lib/categories";

const PAGE_SIZE = 25;

const STATUS_OPTIONS = [
  { value: "", label: "Все статусы" },
  { value: "active", label: "Активные" },
  { value: "cancelled", label: "Отменённые" },
  { value: "completed", label: "Завершённые" },
];

export default function Page() {
  return (
    <AdminGuard allow={["admin", "moderator"]}>
      <EventsList />
    </AdminGuard>
  );
}

function EventsList() {
  const { accessToken, isReady } = useAuth();
  const [events, setEvents] = useState<AdminEventListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [category, setCategory] = useState("");
  const [city, setCity] = useState("");
  const [offset, setOffset] = useState(0);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    if (!isReady || !accessToken) return;
    let cancelled = false;

    (async () => {
      setLoading(true);
      setError(null);
      try {
        const params = new URLSearchParams();
        if (search.trim()) params.set("search", search.trim());
        if (status) params.set("status", status);
        if (category) params.set("category", category);
        if (city.trim()) params.set("city", city.trim());
        params.set("limit", String(PAGE_SIZE));
        params.set("offset", String(offset));

        const res = await apiFetch(`/admin/events?${params.toString()}`, {}, accessToken);
        const json = (await res.json()) as { events: AdminEventListItem[] };
        if (!cancelled) setEvents(json.events ?? []);
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Не удалось загрузить");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [isReady, accessToken, search, status, category, city, offset, reloadKey]);

  const applyFilters = () => {
    setOffset(0);
    setReloadKey((k) => k + 1);
  };

  const resetFilters = () => {
    setSearch("");
    setStatus("");
    setCategory("");
    setCity("");
    setOffset(0);
    setReloadKey((k) => k + 1);
  };

  const hasPrev = offset > 0;
  const hasNext = events.length === PAGE_SIZE;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">События</h1>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setReloadKey((k) => k + 1)}
          disabled={loading}
          className="gap-2"
        >
          {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
          Обновить
        </Button>
      </div>

      {/* Фильтры */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-2">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
          <Input
            placeholder="Поиск по названию"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && applyFilters()}
            className="pl-9"
          />
        </div>
        <select
          value={status}
          onChange={(e) => setStatus(e.target.value)}
          className="h-9 rounded-md border border-input bg-background px-3 text-sm"
        >
          {STATUS_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
        <Input
          placeholder="Город"
          value={city}
          onChange={(e) => setCity(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && applyFilters()}
        />
        <div className="flex gap-2">
          <Button onClick={applyFilters} className="flex-1">
            Найти
          </Button>
          <Button variant="outline" onClick={resetFilters}>
            Сброс
          </Button>
        </div>
      </div>

      {/* Список */}
      {loading && events.length === 0 ? (
        <div className="space-y-2">
          {[1, 2, 3, 4, 5].map((i) => (
            <Skeleton key={i} className="h-16 w-full rounded-lg" />
          ))}
        </div>
      ) : error ? (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <AlertCircle className="h-10 w-10 text-destructive mb-3" />
          <p className="text-sm text-muted-foreground mb-4">{error}</p>
          <Button variant="outline" onClick={() => setReloadKey((k) => k + 1)}>
            Повторить
          </Button>
        </div>
      ) : events.length === 0 ? (
        <div className="py-12 text-center text-sm text-muted-foreground">
          События не найдены
        </div>
      ) : (
        <div className="border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-muted/50 border-b">
              <tr>
                <th className="text-left px-4 py-2 font-medium">Событие</th>
                <th className="text-left px-4 py-2 font-medium hidden md:table-cell">Категория</th>
                <th className="text-left px-4 py-2 font-medium hidden lg:table-cell">Город</th>
                <th className="text-left px-4 py-2 font-medium hidden lg:table-cell">Дата</th>
                <th className="text-left px-4 py-2 font-medium">Статус</th>
                <th className="text-left px-4 py-2 font-medium hidden md:table-cell">Участники</th>
              </tr>
            </thead>
            <tbody>
              {events.map((e) => (
                <tr
                  key={e.id}
                  className="border-b last:border-0 hover:bg-muted/30 transition-colors"
                >
                  <td className="px-4 py-2">
                    <Link
                      href={`/admin/events/${e.id}`}
                      className="font-medium hover:text-primary transition-colors"
                    >
                      {e.title}
                    </Link>
                    <div className="text-xs text-muted-foreground">
                      @{e.organizer_name}
                    </div>
                  </td>
                  <td className="px-4 py-2 hidden md:table-cell">
                    <span className="text-xs">{categoryLabel(e.category)}</span>
                  </td>
                  <td className="px-4 py-2 text-xs hidden lg:table-cell">{e.city}</td>
                  <td className="px-4 py-2 text-xs text-muted-foreground hidden lg:table-cell">
                    {new Date(e.starts_at).toLocaleDateString("ru-RU")}
                  </td>
                  <td className="px-4 py-2">
                    <div className="flex flex-wrap items-center gap-1">
                      <EventStatusBadge status={e.status} />
                      {e.hidden && (
                        <Badge variant="outline" className="text-xs">
                          <EyeOff className="h-3 w-3 mr-1" />
                          скрыто
                        </Badge>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-2 hidden md:table-cell">
                    <span className="text-xs">
                      {e.participants_count}
                      {e.participant_limit ? "" : ""}
                    </span>
                    {e.pending_count > 0 && (
                      <Badge variant="secondary" className="text-xs ml-2">
                        +{e.pending_count}
                      </Badge>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Pagination */}
      <div className="flex items-center justify-between">
        <div className="text-xs text-muted-foreground">
          {events.length > 0 && `Показано ${offset + 1}–${offset + events.length}`}
        </div>
        <div className="flex gap-1">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
            disabled={!hasPrev || loading}
            className="gap-1"
          >
            <ChevronLeft className="h-4 w-4" />
            Назад
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setOffset(offset + PAGE_SIZE)}
            disabled={!hasNext || loading}
            className="gap-1"
          >
            Вперёд
            <ChevronRight className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}

function EventStatusBadge({ status }: { status: "active" | "cancelled" | "completed" }) {
  if (status === "active") {
    return <Badge variant="outline" className="text-xs bg-green-500/10 text-green-700 border-green-500/30 dark:text-green-400">активно</Badge>;
  }
  if (status === "cancelled") {
    return (
      <Badge variant="destructive" className="text-xs">
        <Ban className="h-3 w-3 mr-1" />
        отменено
      </Badge>
    );
  }
  return <Badge variant="secondary" className="text-xs">завершено</Badge>;
}