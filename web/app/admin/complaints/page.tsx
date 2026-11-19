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
  Flag,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { AdminGuard } from "@/components/admin-guard";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { AdminComplaintListItem, AdminComplaintStatus } from "@/lib/types";

const PAGE_SIZE = 25;

const STATUS_OPTIONS: Array<{ value: "" | AdminComplaintStatus; label: string }> = [
  { value: "", label: "Все статусы" },
  { value: "open", label: "Открытые" },
  { value: "resolved", label: "Разрешённые" },
  { value: "rejected", label: "Отклонённые" },
];

const TARGET_LABEL: Record<AdminComplaintListItem["target_type"], string> = {
  event: "событие",
  user: "пользователь",
  message: "сообщение",
};

export default function Page() {
  return (
    <AdminGuard allow={["admin", "moderator"]}>
      <ComplaintsList />
    </AdminGuard>
  );
}

function ComplaintsList() {
  const { accessToken, isReady } = useAuth();
  const [items, setItems] = useState<AdminComplaintListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [status, setStatus] = useState<"" | AdminComplaintStatus>("open");
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
          `/admin/complaints?${params.toString()}`,
          {},
          accessToken
        );
        const json = (await res.json()) as { complaints: AdminComplaintListItem[] };
        if (!cancelled) setItems(json.complaints ?? []);
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
        <h1 className="text-2xl font-bold">Жалобы</h1>
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
          Жалоб нет
        </div>
      ) : (
        <div className="border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-muted/50 border-b">
              <tr>
                <th className="text-left px-4 py-2 font-medium">Цель</th>
                <th className="text-left px-4 py-2 font-medium">Причина</th>
                <th className="text-left px-4 py-2 font-medium hidden md:table-cell">
                  От кого
                </th>
                <th className="text-left px-4 py-2 font-medium">Статус</th>
                <th className="text-left px-4 py-2 font-medium hidden lg:table-cell">
                  Создана
                </th>
              </tr>
            </thead>
            <tbody>
              {items.map((c) => (
                <tr
                  key={c.id}
                  className="border-b last:border-0 hover:bg-muted/30 transition-colors"
                >
                  <td className="px-4 py-2">
                    <Link
                      href={`/admin/complaints/${c.id}`}
                      className="font-medium hover:text-primary transition-colors flex items-center gap-1.5"
                    >
                      <Flag className="h-3 w-3" />
                      {TARGET_LABEL[c.target_type]}
                    </Link>
                    <div className="text-xs text-muted-foreground font-mono truncate max-w-[12ch]">
                      {c.target_id.slice(0, 8)}…
                    </div>
                  </td>
                  <td className="px-4 py-2 max-w-xs">
                    <div className="truncate">{c.reason}</div>
                  </td>
                  <td className="px-4 py-2 text-muted-foreground hidden md:table-cell text-xs">
                    {c.reporter_email ?? "—"}
                  </td>
                  <td className="px-4 py-2">
                    <ComplaintStatusBadge status={c.status} />
                  </td>
                  <td className="px-4 py-2 text-xs text-muted-foreground hidden lg:table-cell">
                    {new Date(c.created_at).toLocaleDateString("ru-RU")}
                  </td>
                </tr>
              ))}
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

function ComplaintStatusBadge({ status }: { status: AdminComplaintStatus }) {
  if (status === "open") {
    return (
      <Badge variant="secondary" className="text-xs">
        <AlertCircle className="h-3 w-3 mr-1" />
        открыта
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
        разрешена
      </Badge>
    );
  }
  return (
    <Badge variant="outline" className="text-xs">
      <XCircle className="h-3 w-3 mr-1" />
      отклонена
    </Badge>
  );
}