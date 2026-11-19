"use client";

import { useAuth } from "@/lib/auth-context";
import { useNotifications, type Notification } from "@/lib/use-notifications";
import { formatDistanceToNow } from "date-fns";
import { ru } from "date-fns/locale";
import { Bell, CheckCheck } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

// Утилита для текста уведомления — та же, что в notification-bell
function getNotificationText(n: Notification): { text: string; href?: string } {
  const p = n.payload || {};
  switch (n.type) {
    case "application_new":
      return {
        text: `Новая заявка на «${p.event_title ?? "событие"}»`,
        href: p.event_id ? `/events/${p.event_id}#applications` : undefined,
      };
    case "application_approved":
      return {
        text: "Ваша заявка одобрена!",
        href: p.event_id ? `/events/${p.event_id}` : undefined,
      };
    case "application_rejected":
      return {
        text: "Ваша заявка отклонена",
        href: p.event_id ? `/events/${p.event_id}` : undefined,
      };
    case "event_cancelled":
      return {
        text: `Событие «${p.event_title ?? "событие"}» отменено`,
        href: p.event_id ? `/events/${p.event_id}` : undefined,
      };
    case "application_cancelled":
      return {
        text: "Участник отменил заявку",
        href: p.event_id ? `/events/${p.event_id}` : undefined,
      };
    default:
      return { text: "Новое уведомление" };
  }
}

export default function NotificationsPage() {
  const { accessToken, isReady } = useAuth();
  const { notifications, unreadCount, loading, markAsRead, markAllAsRead } =
    useNotifications(accessToken);

  if (!isReady) {
    return (
      <div className="max-w-2xl mx-auto py-6 px-4 space-y-3">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-20 w-full rounded-xl" />
        <Skeleton className="h-20 w-full rounded-xl" />
      </div>
    );
  }

  if (!accessToken) {
    return (
      <div className="max-w-2xl mx-auto py-6 px-4 text-center">
        <p className="text-muted-foreground mb-4">
          Войдите, чтобы видеть уведомления.
        </p>
        <Link href="/login" className="text-primary hover:underline">
          Войти
        </Link>
      </div>
    );
  }

  return (
    <div className="max-w-2xl mx-auto py-6 px-4">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl sm:text-3xl font-bold">Уведомления</h1>
        {unreadCount > 0 && (
          <Button
            variant="outline"
            size="sm"
            onClick={markAllAsRead}
            className="gap-2"
          >
            <CheckCheck className="h-4 w-4" />
            Прочитать все
          </Button>
        )}
      </div>

      {loading && notifications.length === 0 && (
        <div className="space-y-3">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-20 w-full rounded-xl" />
          ))}
        </div>
      )}

      {!loading && notifications.length === 0 && (
        <div className="text-center py-16 text-muted-foreground">
          <Bell className="h-12 w-12 mx-auto mb-4 opacity-30" />
          <p>Пока нет уведомлений.</p>
          <p className="text-sm mt-2">
            Когда кто-то подаст заявку на ваше событие или одобрит вашу — здесь появится запись.
          </p>
        </div>
      )}

      <div className="space-y-2">
        {notifications.map((n) => {
          const { text, href } = getNotificationText(n);
          const inner = (
            <div
              className={cn(
                "w-full text-left p-4 rounded-xl border border-border/60 transition-colors",
                !n.is_read
                  ? "bg-primary/5 border-primary/30 hover:bg-primary/10"
                  : "bg-card hover:bg-muted/50"
              )}
            >
              <div className="flex items-start justify-between gap-3">
                <p className={cn("text-sm", !n.is_read && "font-medium")}>
                  {text}
                </p>
                {!n.is_read && (
                  <span className="h-2 w-2 rounded-full bg-primary mt-2 shrink-0" />
                )}
              </div>
              <p className="text-xs text-muted-foreground mt-1.5">
                {formatDistanceToNow(new Date(n.created_at), {
                  addSuffix: true,
                  locale: ru,
                })}
              </p>
            </div>
          );

          return href ? (
            <Link
              key={n.id}
              href={href}
              onClick={() => !n.is_read && markAsRead(n.id)}
              className="block"
            >
              {inner}
            </Link>
          ) : (
            <button
              key={n.id}
              type="button"
              onClick={() => !n.is_read && markAsRead(n.id)}
              className="w-full"
            >
              {inner}
            </button>
          );
        })}
      </div>
    </div>
  );
}