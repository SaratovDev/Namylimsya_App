"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  ArrowLeft,
  Ban,
  ShieldCheck,
  ShieldAlert,
  AlertCircle,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { AdminGuard } from "@/components/admin-guard";
import { BanUserDialog } from "@/components/admin/ban-user-dialog";
import { ChangeRoleDialog } from "@/components/admin/change-role-dialog";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { decodeJwt } from "@/lib/jwt";
import { AdminUserDetails } from "@/lib/types";
import { avatarUrl } from "@/lib/avatars";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";

export default function Page() {
  return (
    <AdminGuard allow={["admin", "moderator"]}>
      <UserDetail />
    </AdminGuard>
  );
}

function UserDetail() {
  const params = useParams();
  const router = useRouter();
  const userId = params.id as string;
  const { accessToken, isReady } = useAuth();
  const [data, setData] = useState<AdminUserDetails | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [actionLoading, setActionLoading] = useState(false);
  const [banOpen, setBanOpen] = useState(false);
  const [roleOpen, setRoleOpen] = useState(false);

  const myRole = accessToken ? (decodeJwt(accessToken)?.role as string | undefined) : null;
  const isAdmin = myRole === "admin";
  const myId = accessToken ? decodeJwt(accessToken)?.user_id : null;
  const isSelf = myId === userId;

  useEffect(() => {
    if (!isReady || !accessToken) return;
    let cancelled = false;

    (async () => {
      setLoading(true);
      setError(null);
      try {
        const res = await apiFetch(`/admin/users/${userId}`, {}, accessToken);
        const json = (await res.json()) as AdminUserDetails;
        if (!cancelled) setData(json);
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Не удалось загрузить");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => { cancelled = true; };
  }, [isReady, accessToken, userId, reloadKey]);

  const reload = useCallback(() => setReloadKey((k) => k + 1), []);

  const handleBan = async (reason: string, expiresAt?: string) => {
    if (!accessToken) return;
    setActionLoading(true);
    try {
      await apiFetch(`/admin/users/${userId}/ban`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ banned: true, reason, expires_at: expiresAt ?? null }),
      }, accessToken);
      toast.success("Пользователь забанен");
      setBanOpen(false);
      reload();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Ошибка");
    } finally {
      setActionLoading(false);
    }
  };

  const handleUnban = async () => {
    if (!accessToken) return;
    if (!window.confirm("Разбанить пользователя?")) return;
    setActionLoading(true);
    try {
      await apiFetch(`/admin/users/${userId}/ban`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ banned: false }),
      }, accessToken);
      toast.success("Пользователь разбанен");
      reload();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Ошибка");
    } finally {
      setActionLoading(false);
    }
  };

  const handleChangeRole = async (role: "user" | "moderator" | "admin") => {
    if (!accessToken) return;
    setActionLoading(true);
    try {
      await apiFetch(`/admin/users/${userId}/role`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ role }),
      }, accessToken);
      toast.success(`Роль изменена на ${role}`);
      setRoleOpen(false);
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
        <Skeleton className="h-48 w-full rounded-xl" />
        <Skeleton className="h-32 w-full rounded-xl" />
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

  const { user, stats } = data;

  return (
    <div className="space-y-4">
      <Button variant="ghost" size="sm" onClick={() => router.back()} className="gap-1 -ml-2">
        <ArrowLeft className="h-4 w-4" />
        Назад
      </Button>

      {/* Профиль */}
      <Card className="p-4 md:p-6">
        <div className="flex items-start gap-4 mb-6">
          <Avatar className="h-14 w-14">
            <AvatarImage src={avatarUrl(user.avatar_url) ?? ""} alt={user.username} />
            <AvatarFallback className="bg-muted">{user.username[0]?.toUpperCase()}</AvatarFallback>
          </Avatar>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <h1 className="text-xl font-bold">@{user.username}</h1>
              <Badge variant={user.role === "admin" ? "default" : user.role === "moderator" ? "secondary" : "outline"}>
                {user.role}
              </Badge>
              {user.is_banned && <Badge variant="destructive">забанен</Badge>}
            </div>
            <p className="text-sm text-muted-foreground">{user.email}</p>
          </div>
        </div>

        <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2 text-sm">
          <Field label="Город" value={user.city} />
          <Field label="Роль" value={user.role} />
          <Field label="Зарегистрирован" value={fmtDate(user.created_at)} />
          <Field label="Согласие" value={user.consent_version ? `v${user.consent_version}` : null} />
          <Field label="Возраст подтверждён" value={user.age_confirmed_at ? fmtDate(user.age_confirmed_at) : null} />
          <Field label="Email подтверждён" value={user.email_verified ? "да" : "нет"} />
          {user.is_banned && (
            <>
              <Field label="Причина бана" value={user.ban_reason} />
              <Field label="Бан истекает" value={user.ban_expires_at ? fmtDate(user.ban_expires_at) : "навсегда"} />
            </>
          )}
        </dl>

        {user.bio && (
          <div className="mt-4">
            <div className="text-xs uppercase tracking-wider text-muted-foreground mb-1">Био</div>
            <p className="text-sm whitespace-pre-wrap">{user.bio}</p>
          </div>
        )}

        {/* Действия */}
        <div className="flex flex-wrap gap-2 mt-6 pt-6 border-t">
          {user.is_banned ? (
            <Button
              variant="outline"
              size="sm"
              onClick={handleUnban}
              disabled={actionLoading}
              className="gap-1"
            >
              <ShieldCheck className="h-4 w-4" />
              Разбанить
            </Button>
          ) : (
            <Button
              variant="destructive"
              size="sm"
              onClick={() => setBanOpen(true)}
              disabled={actionLoading || isSelf}
              className="gap-1"
              title={isSelf ? "Нельзя забанить себя" : undefined}
            >
              <Ban className="h-4 w-4" />
              Забанить
            </Button>
          )}

          {isAdmin && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => setRoleOpen(true)}
              disabled={actionLoading}
              className="gap-1"
            >
              <ShieldAlert className="h-4 w-4" />
              Изменить роль
            </Button>
          )}
        </div>
      </Card>

      {/* Статистика */}
      <Card className="p-4 md:p-6">
        <h2 className="text-sm font-semibold mb-4">Активность</h2>
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
          <StatBox label="Создал событий" value={stats.organized_events} />
          <StatBox label="Заявок всего" value={stats.applications_total} />
          <StatBox label="Одобрено" value={stats.applications_approved} accent="default" />
          <StatBox label="Ожидают" value={stats.applications_pending} accent="warning" />
          <StatBox label="Отклонено" value={stats.applications_rejected} />
          <StatBox label="Отменено" value={stats.applications_cancelled} />
        </div>
      </Card>

      <div className="text-xs text-muted-foreground">
        Ссылки на события и заявки пользователя появятся в B.4/B.5.
      </div>

      <BanUserDialog
        open={banOpen}
        onOpenChange={setBanOpen}
        username={user.username}
        onConfirm={handleBan}
        loading={actionLoading}
      />
      <ChangeRoleDialog
        open={roleOpen}
        onOpenChange={setRoleOpen}
        username={user.username}
        currentRole={user.role}
        onConfirm={handleChangeRole}
        loading={actionLoading}
      />
    </div>
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
      day: "2-digit", month: "2-digit", year: "numeric",
      hour: "2-digit", minute: "2-digit",
    });
  } catch {
    return iso;
  }
}

function StatBox({ label, value, accent }: { label: string; value: number; accent?: "default" | "warning" }) {
  return (
    <div className="rounded-lg border bg-card p-3">
      <div className="text-xs text-muted-foreground mb-1">{label}</div>
      <div className={
        accent === "default" ? "text-xl font-bold text-green-600 dark:text-green-400" :
        accent === "warning" ? "text-xl font-bold text-amber-600 dark:text-amber-400" :
        "text-xl font-bold"
      }>
        {value}
      </div>
    </div>
  );
}