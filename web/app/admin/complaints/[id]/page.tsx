"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  AlertCircle,
  CheckCircle2,
  XCircle,
  Flag,
  Ban,
  Trash2,
  ExternalLink,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { AdminGuard } from "@/components/admin-guard";
import { ComplaintActionDialog } from "@/components/admin/complaint-action-dialog";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { AdminComplaintDetails, AdminComplaintStatus } from "@/lib/types";

export default function Page() {
  return (
    <AdminGuard allow={["admin", "moderator"]}>
      <ComplaintDetail />
    </AdminGuard>
  );
}

function ComplaintDetail() {
  const params = useParams();
  const router = useRouter();
  const id = params.id as string;
  const { accessToken, isReady } = useAuth();

  const [data, setData] = useState<AdminComplaintDetails | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [actionLoading, setActionLoading] = useState(false);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState<"resolve" | "reject">("resolve");

  useEffect(() => {
    if (!isReady || !accessToken) return;
    let cancelled = false;

    (async () => {
      setLoading(true);
      setError(null);
      try {
        const res = await apiFetch(`/admin/complaints/${id}`, {}, accessToken);
        const json = (await res.json()) as AdminComplaintDetails;
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

  const openDialog = (mode: "resolve" | "reject") => {
    setDialogMode(mode);
    setDialogOpen(true);
  };

  const handleAction = async (comment: string) => {
    if (!accessToken) return;
    setActionLoading(true);
    try {
      const endpoint = dialogMode === "resolve" ? "resolve" : "reject";
      await apiFetch(`/admin/complaints/${id}/${endpoint}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ comment }),
      }, accessToken);
      toast.success(
        dialogMode === "resolve" ? "Жалоба разрешена" : "Жалоба отклонена"
      );
      setDialogOpen(false);
      reload();
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

  const c = data.complaint;
  const isOpen = c.status === "open";

  return (
    <div className="space-y-4">
      <Button variant="ghost" size="sm" onClick={() => router.back()} className="gap-1 -ml-2">
        <ArrowLeft className="h-4 w-4" />
        Назад
      </Button>

      {/* Шапка */}
      <Card className="p-4 md:p-6">
        <div className="flex items-start justify-between gap-4 flex-wrap mb-4">
          <div>
            <div className="flex items-center gap-2 mb-2">
              <Badge variant="outline">
                <Flag className="h-3 w-3 mr-1" />
                {c.target_type}
              </Badge>
              <ComplaintStatusBadge status={c.status} />
            </div>
            <h1 className="text-xl font-bold">Жалоба</h1>
            <p className="text-xs text-muted-foreground font-mono mt-1">ID: {c.id}</p>
          </div>

          {isOpen && (
            <div className="flex flex-wrap gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => openDialog("reject")}
                disabled={actionLoading}
                className="gap-1"
              >
                <XCircle className="h-4 w-4" />
                Отклонить
              </Button>
              <Button
                size="sm"
                onClick={() => openDialog("resolve")}
                disabled={actionLoading}
                className="gap-1"
              >
                <CheckCircle2 className="h-4 w-4" />
                Разрешить
              </Button>
            </div>
          )}
        </div>

        <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2 text-sm">
          <Field label="От кого" value={c.reporter_email ?? "—"} />
          <Field label="Дата" value={fmtDate(c.created_at)} />
          {c.resolved_at && (
            <Field label="Рассмотрена" value={fmtDate(c.resolved_at)} />
          )}
        </dl>

        <div className="mt-4 pt-4 border-t">
          <div className="text-xs uppercase tracking-wider text-muted-foreground mb-1">
            Причина жалобы
          </div>
          <p className="text-sm whitespace-pre-wrap">{c.reason}</p>
        </div>

        {c.resolution_comment && (
          <div className="mt-4 pt-4 border-t">
            <div className="text-xs uppercase tracking-wider text-muted-foreground mb-1">
              Комментарий при рассмотрении
            </div>
            <p className="text-sm whitespace-pre-wrap">{c.resolution_comment}</p>
          </div>
        )}
      </Card>

      {/* Цель */}
      {data.event && (
        <Card className="p-4 md:p-6">
          <h2 className="text-sm font-semibold mb-3">Событие</h2>
          <div className="space-y-2 text-sm">
            <div>
              <span className="text-muted-foreground">Название:</span>{" "}
              <span className="font-medium">{data.event.title}</span>
            </div>
            <div>
              <span className="text-muted-foreground">Организатор:</span>{" "}
              <Link
                href={`/admin/users/${data.event.organizer_id}`}
                className="font-medium hover:text-primary"
              >
                @{data.event.organizer_name}
              </Link>
            </div>
            <div>
              <span className="text-muted-foreground">Город:</span> {data.event.city}
            </div>
            <div>
              <span className="text-muted-foreground">Дата:</span>{" "}
              {fmtDate(data.event.starts_at)}
            </div>
            <div>
              <span className="text-muted-foreground">Статус:</span>{" "}
              <Badge variant="outline">{data.event.status}</Badge>
            </div>
          </div>
          <div className="mt-4">
            <Link
              href={`/admin/events/${data.event.id}`}
              className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
            >
              Открыть событие
              <ExternalLink className="h-3.5 w-3.5" />
            </Link>
          </div>
        </Card>
      )}

      {data.user && (
        <Card className="p-4 md:p-6">
          <h2 className="text-sm font-semibold mb-3">Пользователь</h2>
          <div className="space-y-2 text-sm">
            <div>
              <span className="text-muted-foreground">Ник:</span>{" "}
              <span className="font-medium">@{data.user.username}</span>
              {data.user.is_banned && (
                <Badge variant="destructive" className="ml-2 text-xs">
                  <Ban className="h-3 w-3 mr-1" />
                  забанен
                </Badge>
              )}
            </div>
            <div>
              <span className="text-muted-foreground">Email:</span> {data.user.email}
            </div>
            <div>
              <span className="text-muted-foreground">Роль:</span>{" "}
              <Badge variant="outline">{data.user.role}</Badge>
            </div>
          </div>
          <div className="mt-4">
            <Link
              href={`/admin/users/${data.user.id}`}
              className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
            >
              Открыть профиль
              <ExternalLink className="h-3.5 w-3.5" />
            </Link>
          </div>
        </Card>
      )}

      {data.message && (
        <Card className="p-4 md:p-6">
          <h2 className="text-sm font-semibold mb-3">Сообщение</h2>
          <div className="space-y-2 text-sm">
            <div>
              <span className="text-muted-foreground">От:</span>{" "}
              <Link
                href={`/admin/users/${data.message.user_id}`}
                className="font-medium hover:text-primary"
              >
                @{data.message.user_name}
              </Link>
            </div>
            <div>
              <span className="text-muted-foreground">Дата:</span>{" "}
              {fmtDate(data.message.created_at)}
            </div>
            {data.message.deleted_at && (
              <Badge variant="outline" className="text-xs">
                уже удалено
              </Badge>
            )}
          </div>
          <div className="mt-3 p-3 rounded-lg bg-muted/30 border text-sm whitespace-pre-wrap break-words">
            {data.message.text}
          </div>
          <div className="mt-4 flex flex-wrap gap-2">
            <Link
              href={`/admin/events/${data.message.event_id}`}
              className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
            >
              Открыть событие «{data.message.event_title}»
              <ExternalLink className="h-3.5 w-3.5" />
            </Link>
            {!data.message.deleted_at && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => handleDeleteMessage(data.message!.id)}
                className="gap-1 text-destructive hover:text-destructive"
              >
                <Trash2 className="h-4 w-4" />
                Удалить сообщение
              </Button>
            )}
          </div>
        </Card>
      )}

      <ComplaintActionDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        mode={dialogMode}
        onConfirm={handleAction}
        loading={actionLoading}
      />
    </div>
  );
}

function ComplaintStatusBadge({ status }: { status: AdminComplaintStatus }) {
  if (status === "open") {
    return (
      <Badge variant="secondary">
        <AlertCircle className="h-3 w-3 mr-1" />
        открыта
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
        разрешена
      </Badge>
    );
  }
  return (
    <Badge variant="outline">
      <XCircle className="h-3 w-3 mr-1" />
      отклонена
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