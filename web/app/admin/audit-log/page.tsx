"use client";

import { useEffect, useState } from "react";
import {
  AlertCircle,
  Loader2,
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  ChevronUp,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { AdminGuard } from "@/components/admin-guard";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { AdminAuditLogEntry } from "@/lib/types";

const PAGE_SIZE = 50;

// Список известных действий с человеко-читаемым лейблом
const ACTION_LABELS: Record<string, string> = {
  ban_user: "бан пользователя",
  unban_user: "разбан пользователя",
  change_role: "смена роли",
  delete_event: "удаление события",
  cancel_event: "отмена события",
  hide_event: "скрытие события",
  unhide_event: "показ события",
  resolve_complaint: "разрешение жалобы",
  reject_complaint: "отклонение жалобы",
  resolve_ticket: "закрытие тикета",
  reject_ticket: "отклонение тикета",
  reply_ticket: "ответ на тикет",
  delete_message: "удаление сообщения",
};

const TARGET_LABELS: Record<string, string> = {
  user: "пользователь",
  event: "событие",
  message: "сообщение",
  complaint: "жалоба",
  ticket: "тикет",
};

export default function Page() {
  return (
    <AdminGuard allow={["admin"]}>
      <AuditLog />
    </AdminGuard>
  );
}

function AuditLog() {
  const { accessToken, isReady } = useAuth();
  const [entries, setEntries] = useState<AdminAuditLogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [adminIdInput, setAdminIdInput] = useState("");
  const [actionFilter, setActionFilter] = useState("");
  const [targetTypeFilter, setTargetTypeFilter] = useState("");
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
        if (adminIdInput.trim()) params.set("admin_id", adminIdInput.trim());
        if (actionFilter) params.set("action", actionFilter);
        if (targetTypeFilter) params.set("target_type", targetTypeFilter);
        params.set("limit", String(PAGE_SIZE));
        params.set("offset", String(offset));

        const res = await apiFetch(
          `/admin/audit-log?${params.toString()}`,
          {},
          accessToken
        );
        const json = (await res.json()) as { entries: AdminAuditLogEntry[] };
        if (!cancelled) setEntries(json.entries ?? []);
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
  }, [isReady, accessToken, adminIdInput, actionFilter, targetTypeFilter, offset, reloadKey]);

  const applyFilters = () => {
    setOffset(0);
    setReloadKey((k) => k + 1);
  };

  const resetFilters = () => {
    setAdminIdInput("");
    setActionFilter("");
    setTargetTypeFilter("");
    setOffset(0);
    setReloadKey((k) => k + 1);
  };

  const hasPrev = offset > 0;
  const hasNext = entries.length === PAGE_SIZE;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Журнал действий</h1>
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
        <Input
          placeholder="ID администратора (UUID)"
          value={adminIdInput}
          onChange={(e) => setAdminIdInput(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && applyFilters()}
          className="font-mono text-xs"
        />
        <select
          value={actionFilter}
          onChange={(e) => setActionFilter(e.target.value)}
          className="h-9 rounded-md border border-input bg-background px-3 text-sm"
        >
          <option value="">Все действия</option>
          {Object.entries(ACTION_LABELS).map(([v, l]) => (
            <option key={v} value={v}>
              {l}
            </option>
          ))}
        </select>
        <select
          value={targetTypeFilter}
          onChange={(e) => setTargetTypeFilter(e.target.value)}
          className="h-9 rounded-md border border-input bg-background px-3 text-sm"
        >
          <option value="">Все цели</option>
          {Object.entries(TARGET_LABELS).map(([v, l]) => (
            <option key={v} value={v}>
              {l}
            </option>
          ))}
        </select>
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
      {loading && entries.length === 0 ? (
        <div className="space-y-2">
          {[1, 2, 3, 4, 5].map((i) => (
            <Skeleton key={i} className="h-14 w-full rounded-lg" />
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
      ) : entries.length === 0 ? (
        <div className="py-12 text-center text-sm text-muted-foreground">
          Записей нет
        </div>
      ) : (
        <div className="space-y-2">
          {entries.map((e) => (
            <AuditRow key={e.id} entry={e} />
          ))}
        </div>
      )}

      {/* Pagination */}
      <div className="flex items-center justify-between">
        <div className="text-xs text-muted-foreground">
          {entries.length > 0 && `Показано ${offset + 1}–${offset + entries.length}`}
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

function AuditRow({ entry }: { entry: AdminAuditLogEntry }) {
  const [expanded, setExpanded] = useState(false);
  const hasDetails = entry.details && Object.keys(entry.details).length > 0;

  const actionLabel = ACTION_LABELS[entry.action] ?? entry.action;
  const targetLabel = TARGET_LABELS[entry.target_type] ?? entry.target_type;

  return (
    <div className="border rounded-lg p-3 hover:bg-muted/20 transition-colors">
      <div className="flex items-start justify-between gap-3 flex-wrap">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <Badge variant={actionAccent(entry.action)} className="text-xs">
              {actionLabel}
            </Badge>
            <Badge variant="outline" className="text-xs">
              {targetLabel}
            </Badge>
            {entry.target_id && (
              <span className="text-xs text-muted-foreground font-mono truncate max-w-[16ch]">
                {entry.target_id.slice(0, 8)}…
              </span>
            )}
          </div>
          <div className="text-xs text-muted-foreground mt-1.5">
            <span className="font-medium text-foreground">{entry.admin_email}</span>
            {" · "}
            {new Date(entry.created_at).toLocaleString("ru-RU", {
              day: "2-digit",
              month: "2-digit",
              year: "numeric",
              hour: "2-digit",
              minute: "2-digit",
            })}
          </div>
        </div>

        {hasDetails && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setExpanded(!expanded)}
            className="gap-1 text-xs shrink-0"
          >
            {expanded ? (
              <>
                Свернуть <ChevronUp className="h-3 w-3" />
              </>
            ) : (
              <>
                Детали <ChevronDown className="h-3 w-3" />
              </>
            )}
          </Button>
        )}
      </div>

      {expanded && hasDetails && (
        <pre className="mt-2 text-xs bg-muted/50 border rounded p-2 overflow-x-auto whitespace-pre-wrap break-words font-mono">
          {JSON.stringify(entry.details, null, 2)}
        </pre>
      )}
    </div>
  );
}

function actionAccent(action: string): "default" | "secondary" | "destructive" | "outline" {
  if (action.startsWith("ban_") || action.startsWith("delete_") || action.startsWith("reject_")) {
    return "destructive";
  }
  if (action.startsWith("unban_") || action.startsWith("resolve_")) {
    return "secondary";
  }
  if (action.startsWith("change_")) {
    return "default";
  }
  return "outline";
}