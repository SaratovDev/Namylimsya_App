"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  AlertCircle,
  Ban,
  Eye,
  EyeOff,
  Trash2,
  Users,
  MessageSquare,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { AdminGuard } from "@/components/admin-guard";
import { CancelEventDialog } from "@/components/admin/cancel-event-dialog";
import { DeleteEventDialog } from "@/components/admin/delete-event-dialog";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { decodeJwt } from "@/lib/jwt";
import { AdminEventDetails, AdminEventApplication, AdminEventMessage } from "@/lib/types";
import { categoryLabel } from "@/lib/categories";
import { avatarUrl } from "@/lib/avatars";

export default function Page() {
  return (
    <AdminGuard allow={["admin", "moderator"]}>
      <EventDetail />
    </AdminGuard>
  );
}

function EventDetail() {
  const params = useParams();
  const router = useRouter();
  const eventId = params.id as string;
  const { accessToken, isReady } = useAuth();
  const [data, setData] = useState<AdminEventDetails | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [actionLoading, setActionLoading] = useState(false);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const myRole = accessToken ? (decodeJwt(accessToken)?.role as string | undefined) : null;
  const isAdmin = myRole === "admin";

  useEffect(() => {
    if (!isReady || !accessToken) return;
    let cancelled = false;

    (async () => {
      setLoading(true);
      setError(null);
      try {
        const res = await apiFetch(`/admin/events/${eventId}`, {}, accessToken);
        const json = (await res.json()) as AdminEventDetails;
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
  }, [isReady, accessToken, eventId, reloadKey]);

  const reload = useCallback(() => setReloadKey((k) => k + 1), []);

  const handleCancel = async (reason: string) => {
    if (!accessToken) return;
    setActionLoading(true);
    try {
      await apiFetch(`/admin/events/${eventId}/cancel`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ reason }),
      }, accessToken);
      toast.success("Событие отменено");
      setCancelOpen(false);
      reload();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Ошибка");
    } finally {
      setActionLoading(false);
    }
  };

  const handleHideToggle = async () => {
    if (!accessToken || !data) return;
    setActionLoading(true);
    try {
      const action = data.event.hidden ? "unhide" : "hide";
      await apiFetch(`/admin/events/${eventId}/${action}`, { method: "POST" }, accessToken);
      toast.success(data.event.hidden ? "Событие показано" : "Событие скрыто");
      reload();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Ошибка");
    } finally {
      setActionLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!accessToken) return;
    setActionLoading(true);
    try {
      await apiFetch(`/admin/events/${eventId}`, { method: "DELETE" }, accessToken);
      toast.success("Событие удалено");
      setDeleteOpen(false);
      router.replace("/admin/events");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Ошибка");
    } finally {
      setActionLoading(false);
    }
  };

  const handleDeleteMessage = async (messageId: string) => {
    if (!accessToken) return;
    if (!window.confirm("Удалить сообщение?")) return;
    try {
      await apiFetch(`/admin/messages/${messageId}`, { method: "DELETE" }, accessToken);
      toast.success("Сообщение удалено");
      reload();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Ошибка");
    }
  };

  if (loading && !data) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-32" />
        <Skeleton className="h-64 w-full rounded-xl" />
        <Skeleton className="h-64 w-full rounded-xl" />
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

  const { event, applications, messages } = data;
  const isActive = event.status === "active";

  return (
    <div className="space-y-4">
      <Button variant="ghost" size="sm" onClick={() => router.back()} className="gap-1 -ml-2">
        <ArrowLeft className="h-4 w-4" />
        Назад
      </Button>

      {/* Шапка события */}
      <Card className="p-4 md:p-6">
        <div className="flex items-start justify-between gap-4 flex-wrap mb-4">
          <div className="flex-1 min-w-0">
            <div className="flex flex-wrap items-center gap-2 mb-2">
              <Badge variant="outline">{categoryLabel(event.category)}</Badge>
              <EventStatusBadge status={event.status} />
              {event.hidden && (
                <Badge variant="secondary">
                  <EyeOff className="h-3 w-3 mr-1" />
                  скрыто
                </Badge>
              )}
            </div>
            <h1 className="text-xl font-bold break-words">{event.title}</h1>
          </div>

          <div className="flex flex-wrap gap-2">
            {isActive && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setCancelOpen(true)}
                disabled={actionLoading}
                className="gap-1 text-destructive hover:text-destructive"
              >
                <Ban className="h-4 w-4" />
                Отменить
              </Button>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={handleHideToggle}
              disabled={actionLoading}
              className="gap-1"
            >
              {event.hidden ? <Eye className="h-4 w-4" /> : <EyeOff className="h-4 w-4" />}
              {event.hidden ? "Показать" : "Скрыть"}
            </Button>
            {isAdmin && (
              <Button
                variant="destructive"
                size="sm"
                onClick={() => setDeleteOpen(true)}
                disabled={actionLoading}
                className="gap-1"
              >
                <Trash2 className="h-4 w-4" />
                Удалить
              </Button>
            )}
          </div>
        </div>

        <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2 text-sm">
          <Field label="Организатор" value={`@${event.organizer_name} (${event.organizer_email})`} />
          <Field label="Город" value={event.city} />
          <Field label="Адрес" value={event.address} />
          <Field
            label="Координаты"
            value={`${event.lat.toFixed(4)}, ${event.lng.toFixed(4)}${event.hide_exact_location ? " (скрыты)" : ""}`}
          />
          <Field label="Дата" value={fmtDate(event.starts_at)} />
          <Field label="Создано" value={fmtDate(event.created_at)} />
          <Field label="Лимит участников" value={event.participant_limit?.toString() ?? "нет"} />
          <Field label="Участников" value={String(event.participants_count)} />
          <Field label="Pending заявок" value={String(event.pending_count)} />
          <Field label="Отклонённых" value={String(event.rejected_count)} />
          {event.cancel_reason && (
            <Field label="Причина отмены" value={event.cancel_reason} />
          )}
        </dl>

        {event.description && (
          <div className="mt-4 pt-4 border-t">
            <div className="text-xs uppercase tracking-wider text-muted-foreground mb-1">
              Описание
            </div>
            <p className="text-sm whitespace-pre-wrap">{event.description}</p>
          </div>
        )}
      </Card>

      {/* Заявки */}
      <Card className="p-4 md:p-6">
        <div className="flex items-center gap-2 mb-4">
          <Users className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-sm font-semibold">
            Заявки ({applications?.length ?? 0})
          </h2>
        </div>
        {!applications || applications.length === 0 ? (
          <div className="text-sm text-muted-foreground py-4 text-center">
            Заявок нет
          </div>
        ) : (
          <div className="divide-y">
            {applications.map((app) => (
              <div key={app.id} className="flex items-center gap-3 py-2.5 first:pt-0 last:pb-0">
                <Avatar className="h-8 w-8">
                  <AvatarImage src={avatarUrl(app.avatar_url) ?? ""} />
                  <AvatarFallback>{app.username[0]?.toUpperCase()}</AvatarFallback>
                </Avatar>
                <div className="flex-1 min-w-0">
                  <Link
                    href={`/admin/users/${app.user_id}`}
                    className="text-sm font-medium hover:text-primary"
                  >
                    @{app.username}
                  </Link>
                  <div className="text-xs text-muted-foreground">
                    {fmtDate(app.created_at)}
                  </div>
                </div>
                <ApplicationStatusBadge status={app.status} />
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* Сообщения */}
      <Card className="p-4 md:p-6">
        <div className="flex items-center gap-2 mb-4">
          <MessageSquare className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-sm font-semibold">
            Сообщения ({messages?.length ?? 0}
            {event.messages_count > (messages?.length ?? 0) ? ` из ${event.messages_count}` : ""})
          </h2>
        </div>
        {!messages || messages.length === 0 ? (
          <div className="text-sm text-muted-foreground py-4 text-center">
            Сообщений нет
          </div>
        ) : (
          <div className="space-y-2">
            {messages.map((msg) => (
              <MessageRow key={msg.id} msg={msg} onDelete={handleDeleteMessage} />
            ))}
          </div>
        )}
      </Card>

      <CancelEventDialog
        open={cancelOpen}
        onOpenChange={setCancelOpen}
        eventTitle={event.title}
        onConfirm={handleCancel}
        loading={actionLoading}
      />
      <DeleteEventDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        eventTitle={event.title}
        onConfirm={handleDelete}
        loading={actionLoading}
      />
    </div>
  );
}

function MessageRow({
  msg,
  onDelete,
}: {
  msg: AdminEventMessage;
  onDelete: (id: string) => void;
}) {
  return (
    <div className="flex items-start gap-2 p-2 rounded hover:bg-muted/30 group">
      <Avatar className="h-7 w-7 mt-0.5">
        <AvatarImage src={avatarUrl(msg.user_avatar) ?? ""} />
        <AvatarFallback className="text-xs">
          {msg.user_name?.[0]?.toUpperCase()}
        </AvatarFallback>
      </Avatar>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-xs font-medium">@{msg.user_name}</span>
          <span className="text-[10px] text-muted-foreground">
            {fmtDate(msg.created_at)}
          </span>
          {msg.deleted_at && (
            <Badge variant="outline" className="text-[10px]">
              удалено
            </Badge>
          )}
        </div>
        <p className="text-sm break-words mt-0.5">{msg.text}</p>
      </div>
      {!msg.deleted_at && (
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => onDelete(msg.id)}
          title="Удалить сообщение"
          className="opacity-0 group-hover:opacity-100 transition-opacity"
        >
          <X className="h-4 w-4" />
        </Button>
      )}
    </div>
  );
}

function EventStatusBadge({ status }: { status: "active" | "cancelled" | "completed" }) {
  if (status === "active") {
    return (
      <Badge variant="outline" className="bg-green-500/10 text-green-700 border-green-500/30 dark:text-green-400">
        активно
      </Badge>
    );
  }
  if (status === "cancelled") {
    return <Badge variant="destructive">отменено</Badge>;
  }
  return <Badge variant="secondary">завершено</Badge>;
}

function ApplicationStatusBadge({ status }: { status: AdminEventApplication["status"] }) {
  switch (status) {
    case "pending":
      return <Badge variant="secondary" className="text-xs">pending</Badge>;
    case "approved":
      return (
        <Badge variant="outline" className="text-xs bg-green-500/10 text-green-700 border-green-500/30 dark:text-green-400">
          approved
        </Badge>
      );
    case "rejected":
      return <Badge variant="destructive" className="text-xs">rejected</Badge>;
    case "cancelled":
      return <Badge variant="outline" className="text-xs">cancelled</Badge>;
  }
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