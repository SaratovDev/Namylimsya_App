"use client";

import { useState } from "react";
import Link from "next/link";
import { ChevronDown, ChevronUp, MapPin, Users, Share2, X, User } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Button, buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Collapsible, CollapsibleContent } from "@/components/ui/collapsible";
import { EventMap } from "./event-map";
import { useEventActions } from "@/lib/use-event-actions";
import { Event } from "@/lib/types";
import { getEventStatusBadge } from "@/lib/event-utils";
import { useAuth } from "@/lib/auth-context";
import { avatarUrl } from "@/lib/avatars";
import { getUserIdFromToken } from "@/lib/jwt";
import { categoryLabel } from "@/lib/categories";

type Props = {
  event: Event;
  currentUserId?: string | null;
  onRefresh?: () => void;
};

export function EventCard({ event, onRefresh }: Props) {
  const [open, setOpen] = useState(false);
  const { accessToken } = useAuth();
  const { apply, cancelApplication, share, loading } = useEventActions(
    event.id,
    event.user_application_id,
    onRefresh
  );

  const appStatus = event.user_application_status;
  const userId = accessToken ? getUserIdFromToken(accessToken) : null;
  const isOrganizer = !!userId && event.organizer_id === userId;
  const isInactive = event.status !== "active";

  const dateObj = new Date(event.starts_at);
  const day = dateObj.toLocaleDateString("ru-RU", { day: "2-digit" });
  const month = dateObj.toLocaleDateString("ru-RU", { month: "long" });
  const time = dateObj.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
  const statusBadge = getEventStatusBadge(event);

  // Клик по карточке раскрывает/сворачивает (но не если клик по ссылке/кнопке/организатору)
  const handleCardClick = () => {
    setOpen((v) => !v);
  };

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <Card
        className="grid grid-cols-1 sm:grid-cols-[180px_1fr] overflow-hidden border-border/50 hover:shadow-md transition-shadow bg-card [--card-spacing:0px] cursor-pointer"
        onClick={handleCardClick}
      >
        {/* ЛЕВАЯ КОЛОНКА (Дата) */}
        <div className="p-4 sm:p-6 flex flex-row sm:flex-col items-center sm:items-start justify-between sm:justify-center border-b sm:border-b-0 sm:border-r border-border/50 bg-muted/10 gap-4 sm:gap-0">
          <div className="flex items-center gap-2 sm:block">
            <div className="text-2xl sm:text-4xl font-light tracking-tight flex items-baseline gap-1">
              {day} <span className="text-base sm:text-xl font-normal">{month}</span>
            </div>
            <div className="text-2xl sm:text-4xl font-light sm:mt-1 tracking-tighter">
              {time}
            </div>
          </div>
          {statusBadge && (
            <div className="text-xs font-medium uppercase tracking-wider text-muted-foreground sm:mt-3">
              {statusBadge.text}
            </div>
          )}
        </div>

        {/* ПРАВАЯ КОЛОНКА (Контент) */}
        <div className="p-6 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-6">
              <Badge variant="secondary" className="bg-secondary text-secondary-foreground hover:bg-secondary/80 rounded-full px-3 py-0.5 text-xs font-medium uppercase tracking-wide">
                {categoryLabel(event.category)}
              </Badge>
            </div>

            {/* Заголовок — ведёт на страницу события, не раскрывает карточку */}
            <Link
              href={`/events/${event.id}`}
              className="block group/title"
              onClick={(e) => e.stopPropagation()}
            >
              <h3 className="text-2xl font-semibold mb-4 group-hover/title:text-primary transition-colors leading-tight">
                {event.title}
              </h3>
            </Link>

            {event.description && (
              <p className="text-sm text-muted-foreground line-clamp-2 mb-4 max-w-120">
                {event.description}
              </p>
            )}

            {event.status === "cancelled" && event.cancel_reason && (
              <p className="text-sm text-destructive/80 mb-4">
                Причина отмены: {event.cancel_reason}
              </p>
            )}

            <div className="flex items-center gap-3 mb-6">
              {/* Организатор — кликабельная ссылка на профиль */}
              <Link
                href={`/users/${event.organizer_id}`}
                className="flex items-center gap-3 group/org"
                onClick={(e) => e.stopPropagation()}
              >
                <Avatar className="h-8 w-8 bg-muted border border-border/50">
                  <AvatarImage src={avatarUrl(event.organizer_avatar) ?? ""} alt={event.organizer_name} />
                  <AvatarFallback className="bg-transparent text-muted-foreground">
                    <User className="h-4 w-4" />
                  </AvatarFallback>
                </Avatar>

                <div className="flex flex-col">
                  <span className="text-sm font-semibold leading-tight group-hover/org:text-primary transition-colors">
                    {event.organizer_name}
                  </span>
                  <span className="text-xs text-muted-foreground mt-0.5">Организатор</span>
                </div>
              </Link>

              {isOrganizer && event.pending_applications_count > 0 && (
                <Badge variant="destructive" className="text-xs px-2 py-0.5">
                  {event.pending_applications_count} новых
                </Badge>
              )}
            </div>
          </div>

          <div>
            <div className="flex items-end gap-3">
              <div className="flex-1" onClick={(e) => e.stopPropagation()}>
                {isInactive ? (
                  <div className={buttonVariants({
                    variant: "secondary",
                    className: "w-full pointer-events-none opacity-80",
                  })}>
                    {event.status === "cancelled" ? "Событие отменено" : "Событие завершено"}
                  </div>
                ) : isOrganizer ? (
                  <div className="flex gap-2">
                    <div className={buttonVariants({ variant: "secondary", className: "flex-1 pointer-events-none opacity-80" })}>
                      Это ваше событие
                    </div>
                    <Link href={`/events/${event.id}#applications`} className={buttonVariants({ variant: "outline" })}>
                      Заявки
                    </Link>
                    <Link href={`/events/${event.id}`} className={buttonVariants({ variant: "default" })}>
                      Чат
                    </Link>
                  </div>
                ) : appStatus === "pending" ? (
                  <div className="flex gap-2">
                    <div className={buttonVariants({ variant: "secondary", className: "flex-1 pointer-events-none opacity-80" })}>
                      Вы ожидаете одобрения
                    </div>
                    <Link href={`/events/${event.id}`} className={buttonVariants({ variant: "outline" })}>
                      В чат
                    </Link>
                  </div>
                ) : appStatus === "approved" ? (
                  <div className="flex gap-2">
                    <div className={buttonVariants({ className: "flex-1 pointer-events-none bg-green-600/20 text-green-500 hover:bg-green-600/20 border border-green-600/30" })}>
                      Вы сюда намылились
                    </div>
                    <Link href={`/events/${event.id}`} className={buttonVariants({ variant: "outline" })}>
                      В чат
                    </Link>
                  </div>
                ) : (
                  <Button className="w-full h-10" onClick={apply} disabled={loading}>
                    {loading ? "Отправка..." : "Хочу участвовать"}
                  </Button>
                )}
              </div>

              <div className="flex items-center gap-1.5 text-sm text-muted-foreground h-10 px-2">
                <Users className="h-4 w-4" />
                <span className="font-medium text-foreground">{event.participants_count}</span>
                {event.participant_limit && <span>/ {event.participant_limit}</span>}
              </div>
            </div>

            <div className="flex items-center justify-between mt-4">
              <div className="flex items-center gap-1.5 text-xs text-muted-foreground uppercase tracking-wider">
                <MapPin className="h-3 w-3" />
                {event.city}
              </div>

              {!isInactive && (
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    setOpen((v) => !v);
                  }}
                  className="text-xs text-muted-foreground hover:text-foreground flex items-center gap-1 transition-colors"
                >
                  {open ? "Свернуть" : "Подробнее"}
                  {open ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
                </button>
              )}
            </div>
          </div>
        </div>

        {/* РАСКРЫВАЮЩИЙСЯ КОНТЕНТ */}
        <div className="col-span-1 sm:col-span-2">
          <CollapsibleContent>
            <div className="p-6 pt-0 border-t border-border/50" onClick={(e) => e.stopPropagation()}>
              <div className="pt-6 space-y-4">
                <EventMap
                  lat={event.lat}
                  lng={event.lng}
                  city={event.city}
                  hideExactLocation={event.hide_exact_location}
                  canSeeExact={event.can_see_exact_location ?? false}
                />
                <div className="flex gap-2">
                  <Button variant="outline" size="sm" className="flex-1 gap-2" onClick={share}>
                    <Share2 className="h-4 w-4" />
                    Поделиться
                  </Button>
                  {(appStatus === "pending" || appStatus === "approved") && (
                    <Button
                      variant="outline"
                      size="sm"
                      className="flex-1 gap-2 text-destructive hover:text-destructive"
                      onClick={cancelApplication}
                      disabled={loading}
                    >
                      <X className="h-4 w-4" />
                      Отменить заявку
                    </Button>
                  )}
                </div>
              </div>
            </div>
          </CollapsibleContent>
        </div>
      </Card>
    </Collapsible>
  );
}