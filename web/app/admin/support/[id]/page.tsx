"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  AlertCircle,
  CheckCircle2,
  XCircle,
  Mail,
  MessageSquare,
  Send,
  Flag,
  ExternalLink,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { AdminGuard } from "@/components/admin-guard";
import { TicketActionDialog } from "@/components/admin/ticket-action-dialog";
import { ReplyTicketDialog } from "@/components/admin/reply-ticket-dialog";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { AdminTicketDetails, AdminTicketStatus, AdminTicketType } from "@/lib/types";

const TYPE_LABEL: Record<AdminTicketType, string> = {
  complaint: "Жалоба",
  suggestion: "Предложение",
  appeal: "Апелляция",
};

export default function Page() {
  return (
    <AdminGuard allow={["admin", "moderator"]}>
      <TicketDetail />
    </AdminGuard>
  );
}

function TicketDetail() {
  const params = useParams();
  const router = useRouter();
  const id = params.id as string;
  const { accessToken, isReady } = useAuth();

  const [data, setData] = useState<AdminTicketDetails | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [actionLoading, setActionLoading] = useState(false);

  const [actionOpen, setActionOpen] = useState(false);
  const [actionMode, setActionMode] = useState<"resolve" | "reject">("resolve");
  const [replyOpen, setReplyOpen] = useState(false);

  useEffect(() => {
    if (!isReady || !accessToken) return;
    let cancelled = false;

    (async () => {
      setLoading(true);
      setError(null);
      try {
        const res = await apiFetch(`/admin/support/tickets/${id}`, {}, accessToken);
        const json = (await res.json()) as AdminTicketDetails;
        if (!cancelled) setData(json);
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
  }, [isReady, accessToken, id, reloadKey]);

  const reload = useCallback(() => setReloadKey((k) => k + 1), []);

  const openAction = (mode: "resolve" | "reject") => {
    setActionMode(mode);
    setActionOpen(true);
  };

  const handleAction = async (comment: string) => {
    if (!accessToken) return;
    setActionLoading(true);
    try {
      const endpoint = actionMode === "resolve" ? "resolve" : "reject";
      await apiFetch(`/admin/support/tickets/${id}/${endpoint}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ comment }),
      }, accessToken);
      toast.success(
        actionMode === "resolve" ? "Тикет закрыт" : "Тикет отклонён"
      );
      setActionOpen(false);
      reload();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Ошибка");
    } finally {
      setActionLoading(false);
    }
  };

  const handleReply = async (body: string) => {
    if (!accessToken) return;
    setActionLoading(true);
    try {
      await apiFetch(`/admin/support/tickets/${id}/reply`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ body }),
      }, accessToken);
      toast.success("Ответ отправлен");
      setReplyOpen(false);
      reload();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Ошибка");
    } finally {
      setActionLoading(false);
    }
  };

  if (loading && !data) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-32" />
        <Skeleton className="h-64 w-full rounded-xl" />
        <Skeleton className="h-48 w-full rounded-xl" />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <AlertCircle className="h-10 w-10 text-destructive mb-3" />
        <p className="text-sm text-muted-foreground mb-4">{error ?? "Нет данных"}</p>
        <div className="flex gap-2">
          <Button variant="ghost" onClick={() => router.back()}>Назад</Button>
          <Button variant="outline" onClick={reload}>Повторить</Button>
        </div>
      </div>
    );
  }

  const t = data.ticket;
  const isOpen = t.status === "open";

  return (
    <div className="space-y-4">
      <Button variant="ghost" size="sm" onClick={() => router.back()} className="gap-1 -ml-2">
        <ArrowLeft className="h-4 w-4" />
        Назад
      </Button>

      {/* Тикет */}
      <Card className="p-4 md:p-6">
        <div className="flex items-start justify-between gap-4 flex-wrap mb-4">
          <div>
            <div className="flex items-center gap-2 mb-2">
              <Badge variant="outline">{TYPE_LABEL[t.type]}</Badge>
              <TicketStatusBadge status={t.status} />
            </div>
            <h1 className="text-xl font-bold break-words">
              {t.subject || "(без темы)"}
            </h1>
            <p className="text-xs text-muted-foreground font-mono mt-1">
              ID: {t.id}
            </p>
          </div>

          {isOpen && (
            <div className="flex flex-wrap gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setReplyOpen(true)}
                disabled={actionLoading}
                className="gap-1"
              >
                <Send className="h-4 w-4" />
                Ответить
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => openAction("reject")}
                disabled={actionLoading}
                className="gap-1"
              >
                <XCircle className="h-4 w-4" />
                Отклонить
              </Button>
              <Button
                size="sm"
                onClick={() => openAction("resolve")}
                disabled={actionLoading}
                className="gap-1"
              >
                <CheckCircle2 className="h-4 w-4" />
                Закрыть
              </Button>
            </div>
          )}
        </div>

        <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2 text-sm">
          <Field
            label="Пользователь"
            value={
              t.user_id
                ? t.user_username
                  ? `@${t.user_username} (${t.user_email ?? "—"})`
                  : t.user_email ?? "—"
                : "аноним"
            }
          />
          <Field label="Дата" value={fmtDate(t.created_at)} />
          {t.resolved_at && (
            <Field label="Рассмотрен" value={fmtDate(t.resolved_at)} />
          )}
          {t.complaint_id && (
            <Field label="Связан с жалобой" value={t.complaint_id.slice(0, 8) + "…"} />
          )}
        </dl>

        <div className="mt-4 pt-4 border-t">
          <div className="text-xs uppercase tracking-wider text-muted-foreground mb-1">
            Сообщение
          </div>
          <p className="text-sm whitespace-pre-wrap break-words">{t.message}</p>
        </div>

        {t.resolution_comment && (
          <div className="mt-4 pt-4 border-t">
            <div className="text-xs uppercase tracking-wider text-muted-foreground mb-1">
              Комментарий при закрытии
            </div>
            <p className="text-sm whitespace-pre-wrap">{t.resolution_comment}</p>
          </div>
        )}

        <div className="mt-4 flex flex-wrap gap-3">
          {t.user_id && (
            <Link
              href={`/admin/users/${t.user_id}`}
              className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
            >
              Открыть профиль
              <ExternalLink className="h-3.5 w-3.5" />
            </Link>
          )}
          {t.complaint_id && (
            <Link
              href={`/admin/complaints/${t.complaint_id}`}
              className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
            >
              <Flag className="h-3.5 w-3.5" />
              Открыть жалобу
            </Link>
          )}
        </div>
      </Card>

      {/* Ответы */}
      <Card className="p-4 md:p-6">
        <div className="flex items-center gap-2 mb-4">
          <MessageSquare className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-sm font-semibold">
            Ответы ({data.replies.length})
          </h2>
        </div>

        {data.replies.length === 0 ? (
          <div className="text-sm text-muted-foreground py-4 text-center">
            Пока нет ответов
          </div>
        ) : (
          <div className="space-y-3">
            {data.replies.map((r) => (
              <div
                key={r.id}
                className="p-3 rounded-lg border bg-muted/20"
              >
                <div className="flex items-center justify-between gap-2 mb-2">
                  <div className="flex items-center gap-2 text-xs">
                    <Mail className="h-3 w-3 text-muted-foreground" />
                    <span className="font-medium">{r.admin_email || "—"}</span>
                  </div>
                  <span className="text-[10px] text-muted-foreground">
                    {fmtDate(r.created_at)}
                  </span>
                </div>
                <p className="text-sm whitespace-pre-wrap break-words">{r.body}</p>
              </div>
            ))}
          </div>
        )}
      </Card>

      <TicketActionDialog
        open={actionOpen}
        onOpenChange={setActionOpen}
        mode={actionMode}
        onConfirm={handleAction}
        loading={actionLoading}
      />
      <ReplyTicketDialog
        open={replyOpen}
        onOpenChange={setReplyOpen}
        userEmail={t.user_email}
        subject={t.subject || "(без темы)"}
        onConfirm={handleReply}
        loading={actionLoading}
      />
    </div>
  );
}

function TicketStatusBadge({ status }: { status: AdminTicketStatus }) {
  if (status === "open") {
    return (
      <Badge variant="secondary">
        <AlertCircle className="h-3 w-3 mr-1" />
        открыт
      </Badge>
    );
  }
  if (status === "resolved") {
    return (
      <Badge
        variant="outline"
        className="bg-green-500/10 text-green-700 border-green-500/30 dark:text-green-400"
      >
        <CheckCircle2 className="h-3 w-3 mr-1" />
        решён
      </Badge>
    );
  }
  return (
    <Badge variant="outline">
      <XCircle className="h-3 w-3 mr-1" />
      отклонён
    </Badge>
  );
}

function Field({ label, value }: { label: string; value: string | null | undefined }) {
  return (
    <>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="font-medium break-words">{value ?? "—"}</dd>
    </>
  );
}

function fmtDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString("ru-RU", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return iso;
  }
}