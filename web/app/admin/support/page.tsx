"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import {
  AlertCircle,
  Loader2,
  ChevronLeft,
  ChevronRight,
  CheckCircle2,
  XCircle,
  Mail,
  Lightbulb,
  ShieldAlert,
  Flag,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { AdminGuard } from "@/components/admin-guard";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { AdminTicketListItem, AdminTicketStatus, AdminTicketType } from "@/lib/types";

const PAGE_SIZE = 25;

const STATUS_OPTIONS: Array<{ value: "" | AdminTicketStatus; label: string }> = [
  { value: "", label: "Все статусы" },
  { value: "open", label: "Открытые" },
  { value: "resolved", label: "Решённые" },
  { value: "rejected", label: "Отклонённые" },
];

const TYPE_META: Record<
  AdminTicketType,
  { label: string; icon: React.ComponentType<{ className?: string }> }
> = {
  complaint: { label: "жалоба", icon: Flag },
  suggestion: { label: "предложение", icon: Lightbulb },
  appeal: { label: "апелляция", icon: ShieldAlert },
};

export default function Page() {
  return (
    <AdminGuard allow={["admin", "moderator"]}>
      <TicketsList />
    </AdminGuard>
  );
}

function TicketsList() {
  const { accessToken, isReady } = useAuth();
  const [items, setItems] = useState<AdminTicketListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [status, setStatus] = useState<"" | AdminTicketStatus>("open");
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
        if (status) params.set("status", status);
        params.set("limit", String(PAGE_SIZE));
        params.set("offset", String(offset));

        const res = await apiFetch(
          `/admin/support/tickets?${params.toString()}`,
          {},
          accessToken
        );
        const json = (await res.json()) as { tickets: AdminTicketListItem[] };
        if (!cancelled) setItems(json.tickets ?? []);
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
  }, [isReady, accessToken, status, offset, reloadKey]);

  const hasPrev = offset > 0;
  const hasNext = items.length === PAGE_SIZE;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Поддержка</h1>
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

      <div className="flex items-center gap-2">
        <select
          value={status}
          onChange={(e) => {
            setStatus(e.target.value as typeof status);
            setOffset(0);
          }}
          className="h-9 rounded-md border border-input bg-background px-3 text-sm"
        >
          {STATUS_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </div>

      {loading && items.length === 0 ? (
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
      ) : items.length === 0 ? (
        <div className="py-12 text-center text-sm text-muted-foreground">
          Тикетов нет
        </div>
      ) : (
        <div className="border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-muted/50 border-b">
              <tr>
                <th className="text-left px-4 py-2 font-medium">Тип</th>
                <th className="text-left px-4 py-2 font-medium">Тема</th>
                <th className="text-left px-4 py-2 font-medium hidden md:table-cell">
                  Пользователь
                </th>
                <th className="text-left px-4 py-2 font-medium">Статус</th>
                <th className="text-left px-4 py-2 font-medium hidden lg:table-cell">
                  Создан
                </th>
              </tr>
            </thead>
            <tbody>
              {items.map((t) => {
                const meta = TYPE_META[t.type];
                const Icon = meta.icon;
                return (
                  <tr
                    key={t.id}
                    className="border-b last:border-0 hover:bg-muted/30 transition-colors"
                  >
                    <td className="px-4 py-2">
                      <Link
                        href={`/admin/support/${t.id}`}
                        className="font-medium hover:text-primary transition-colors inline-flex items-center gap-1.5"
                      >
                        <Icon className="h-3 w-3" />
                        {meta.label}
                      </Link>
                    </td>
                    <td className="px-4 py-2 max-w-xs">
                      <Link
                        href={`/admin/support/${t.id}`}
                        className="hover:text-primary transition-colors"
                      >
                        <div className="truncate">{t.subject || "(без темы)"}</div>
                      </Link>
                    </td>
                    <td className="px-4 py-2 text-muted-foreground hidden md:table-cell text-xs">
                      {t.user_email ? (
                        <span className="inline-flex items-center gap-1">
                          <Mail className="h-3 w-3" />
                          {t.user_email}
                        </span>
                      ) : (
                        "аноним"
                      )}
                    </td>
                    <td className="px-4 py-2">
                      <TicketStatusBadge status={t.status} />
                    </td>
                    <td className="px-4 py-2 text-xs text-muted-foreground hidden lg:table-cell">
                      {new Date(t.created_at).toLocaleDateString("ru-RU")}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <div className="flex items-center justify-between">
        <div className="text-xs text-muted-foreground">
          {items.length > 0 && `Показано ${offset + 1}–${offset + items.length}`}
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

function TicketStatusBadge({ status }: { status: AdminTicketStatus }) {
  if (status === "open") {
    return (
      <Badge variant="secondary" className="text-xs">
        <AlertCircle className="h-3 w-3 mr-1" />
        открыт
      </Badge>
    );
  }
  if (status === "resolved") {
    return (
      <Badge
        variant="outline"
        className="text-xs bg-green-500/10 text-green-700 border-green-500/30 dark:text-green-400"
      >
        <CheckCircle2 className="h-3 w-3 mr-1" />
        решён
      </Badge>
    );
  }
  return (
    <Badge variant="outline" className="text-xs">
      <XCircle className="h-3 w-3 mr-1" />
      отклонён
    </Badge>
  );
}