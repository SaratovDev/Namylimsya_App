"use client";

import { Bell } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Badge } from "@/components/ui/badge";
import { useNotifications, type Notification } from "@/lib/use-notifications";
import { useAuth } from "@/lib/auth-context";
import { formatDistanceToNow } from "date-fns";
import { ru } from "date-fns/locale";

export function NotificationBell() {
  const { accessToken } = useAuth();
  const { notifications, unreadCount, markAsRead, markAllAsRead } = useNotifications(accessToken);

  // Функция для текста уведомления
  const getText = (n: Notification) => {
    switch (n.type) {
      case "application_new": return `Новая заявка на «${n.payload.event_title}»`;
      case "application_approved": return `Ваша заявка одобрена!`;
      case "application_rejected": return `Заявка отклонена`;
      case "event_cancelled": return `Событие «${n.payload.event_title}» отменено`;
      case "event_updated": return `Событие «${n.payload.event_title ?? "событие"}» изменено`;
      case "application_cancelled": return `Участник отменил заявку`;
      default: return "Новое уведомление";
    }
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="relative h-10 w-10 rounded-full inline-flex items-center justify-center hover:bg-accent/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
        <Bell className="h-5 w-5" />
        {unreadCount > 0 && (
          <Badge className="absolute -top-1 -right-1 h-5 min-w-5 px-1.5 bg-destructive text-destructive-foreground text-xs flex items-center justify-center">
            {unreadCount > 99 ? "99+" : unreadCount}
          </Badge>
        )}
      </DropdownMenuTrigger>

      <DropdownMenuContent align="end" className="w-80">
        <div className="flex items-center justify-between px-2 py-1.5">
          <span className="font-semibold text-sm">Уведомления</span>
          {unreadCount > 0 && (
            <button 
              onClick={markAllAsRead}
              className="text-xs text-primary hover:underline"
            >
              Прочитать все
            </button>
          )}
        </div>
        <DropdownMenuSeparator />
        
        <div className="max-h-[300px] overflow-y-auto">
          {notifications.length === 0 ? (
            <div className="p-4 text-center text-sm text-muted-foreground">
              Нет уведомлений
            </div>
          ) : (
            notifications.map((n) => (
              <div 
                key={n.id} 
                onClick={() => !n.is_read && markAsRead(n.id)}
                className={`p-3 border-b last:border-0 text-sm cursor-pointer hover:bg-muted/50 transition-colors ${
                  !n.is_read ? "bg-primary/5" : ""
                }`}
              >
                <p className="font-medium mb-1">{getText(n)}</p>
                <p className="text-xs text-muted-foreground">
                  {formatDistanceToNow(new Date(n.created_at), { addSuffix: true, locale: ru })}
                </p>
              </div>
            ))
          )}
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}