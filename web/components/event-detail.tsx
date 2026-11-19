"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { MapPin, Calendar, ChevronLeft, Users, X, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { EventMap } from "./event-map";
import { EventChat } from "./event-chat";
import { useAuth } from "@/lib/auth-context";
import { Event } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { EventApplicationsList } from "./event-applications-list";
import { avatarUrl } from "@/lib/avatars";
import { categoryLabel } from "@/lib/categories";
import { getUserIdFromToken } from "@/lib/jwt";
import { apiFetch } from "@/lib/api";
import { CancelDialog } from "./cancel-dialog";
import { ParticipantsList } from "./participants-list";
import Link from "next/link";

type EventDetailData = Event & {
  organizer_name?: string;
  can_see_exact_location?: boolean;
};

type Props = {
  event: EventDetailData;
  onRefresh?: () => void;
};

export function EventDetail({ event, onRefresh }: Props) {
  const router = useRouter();
  const { accessToken } = useAuth();

  const userId = accessToken ? getUserIdFromToken(accessToken) : null;
  const isOrganizer = !!userId && event.organizer_id === userId;
  const appStatus = event.user_application_status || "";
  const isCancelled = event.status === "cancelled";
  const isCompleted = event.status === "completed";
  const isInactive = isCancelled || isCompleted;

  const canChat =
    !isInactive &&
    (isOrganizer || appStatus === "approved" || appStatus === "pending");

  const canSeeExact = event.can_see_exact_location;

  // Локальные состояния для кнопок действий
  const [applying, setApplying] = useState(false);
  const [cancellingApp, setCancellingApp] = useState(false);
  const [cancellingEvent, setCancellingEvent] = useState(false);
    const [cancelEventDialogOpen, setCancelEventDialogOpen] = useState(false);
  const [cancelAppDialogOpen, setCancelAppDialogOpen] = useState(false);

  const dateObj = new Date(event.starts_at);
  const dateString =
    dateObj.toLocaleDateString("ru-RU", {
      day: "numeric",
      month: "long",
      year: "numeric",
    }) + "г.";

  // ─── Действия ───────────────────────────────────────────────────────

  const handleApply = async () => {
    if (!accessToken) {
      toast.error("Войдите, чтобы подать заявку");
      return;
    }
    setApplying(true);
    try {
      await apiFetch(
        `/events/${event.id}/applications`,
        { method: "POST" },
        accessToken
      );
      toast.success("Заявка отправлена!");
      onRefresh?.();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Не удалось подать заявку"
      );
    } finally {
      setApplying(false);
    }
  };

  const handleCancelApplication = async (reason: string) => {
    if (!accessToken || !event.user_application_id) return;
    setCancellingApp(true);
    try {
      await apiFetch(
        `/applications/${event.user_application_id}/cancel`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ reason }),
        },
        accessToken
      );
      toast.success("Заявка отменена");
      setCancelAppDialogOpen(false);
      onRefresh?.();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Не удалось отменить заявку"
      );
    } finally {
      setCancellingApp(false);
    }
  };

  const handleCancelEvent = async (reason: string) => {
    if (!accessToken) return;
    setCancellingEvent(true);
    try {
      await apiFetch(
        `/events/${event.id}/cancel`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ reason }),
        },
        accessToken
      );
      toast.success("Событие отменено");
      setCancelEventDialogOpen(false);
      onRefresh?.();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Не удалось отменить событие"
      );
    } finally {
      setCancellingEvent(false);
    }
  };

  // ─── Рендер ─────────────────────────────────────────────────────────

  return (
    <div className="w-full max-w-[1600px] mx-auto py-2 sm:py-6 px-0 sm:px-4">
      <Button
        variant="ghost"
        size="sm"
        onClick={() => router.back()}
        className="mb-4 ml-2 sm:-ml-2 text-muted-foreground"
      >
        <ChevronLeft className="h-4 w-4 mr-1" />
        Назад
      </Button>

      <div className="grid grid-cols-1 lg:grid-cols-[1fr_340px] gap-4 sm:gap-6">
        {/* ЛЕВАЯ КОЛОНКА: Инфо + Чат */}
        <div className="flex flex-col gap-6">
          <div className="border rounded-xl p-4 sm:p-6 bg-card">
 {/* Шапка: badges + действия в одной строке */}
<div className="flex flex-wrap items-start justify-between gap-3 mb-3">
  <div className="flex flex-wrap items-center gap-2">
    <Badge
      variant="outline"
      className="uppercase tracking-wider text-xs px-3 py-1"
    >
      {categoryLabel(event.category)}
    </Badge>
    {isCancelled && <Badge variant="destructive">Отменено</Badge>}
    {isCompleted && <Badge variant="secondary">Завершено</Badge>}
  </div>

  {!isInactive && (
    <div className="flex flex-wrap gap-2 shrink-0">
      {isOrganizer ? (
        <Button
          variant="outline"
          size="sm"
          onClick={() => setCancelEventDialogOpen(true)}
          disabled={cancellingEvent}
          className="text-destructive hover:text-destructive"
        >
          {cancellingEvent ? (
            <Loader2 className="h-4 w-4 mr-1 animate-spin" />
          ) : (
            <X className="h-4 w-4 mr-1" />
          )}
          Отменить событие
        </Button>
      ) : appStatus === "pending" || appStatus === "approved" ? (
        <Button
          variant="outline"
          size="sm"
          onClick={() => setCancelAppDialogOpen(true)}
          disabled={cancellingApp}
          className="text-destructive hover:text-destructive"
        >
          {cancellingApp ? (
            <Loader2 className="h-4 w-4 mr-1 animate-spin" />
          ) : (
            <X className="h-4 w-4 mr-1" />
          )}
          Отменить заявку
        </Button>
      ) : (
        <Button
          size="sm"
          onClick={handleApply}
          disabled={applying}
        >
          {applying && <Loader2 className="h-4 w-4 mr-1 animate-spin" />}
          {applying ? "Отправка..." : "Хочу участвовать"}
        </Button>
      )}
    </div>
  )}
</div>

{/* Заголовок */}
    <h1 className="text-2xl font-bold mb-2">{event.title}</h1>
{/* Дата */}
<div className="flex items-center gap-2 text-muted-foreground text-sm mb-4">
  <Calendar className="h-4 w-4" />
  <span>{dateString}</span>
</div>

{/* Орг слева + участники справа */}
<div className="flex flex-wrap items-center justify-between gap-3">
        <Link
          href={`/users/${event.organizer_id}`}
          className="flex items-center gap-3 group/org w-fit"
        >
          <Avatar className="h-10 w-10">
            <AvatarImage src={avatarUrl(event.organizer_avatar) ?? ""} />
            <AvatarFallback className="bg-muted">
              <span className="text-muted-foreground text-xs">?</span>
            </AvatarFallback>
          </Avatar>
          <div>
            <p className="font-semibold text-sm group-hover/org:text-primary transition-colors">
              @{event.organizer_name}
            </p>
            <p className="text-xs text-muted-foreground mt-0.5">Организатор</p>
          </div>
        </Link>

  <div className="flex items-center gap-1.5 text-sm text-muted-foreground">
    <Users className="h-4 w-4" />
    <span className="font-medium text-foreground">
      {event.participants_count}
    </span>
    {event.participant_limit && <span>/ {event.participant_limit}</span>}
    <span className="ml-1">участников</span>
  </div>
</div>

{/* Причина отмены — под футером, на всю ширину */}
{isCancelled && event.cancel_reason && (
  <div className="mt-4 p-3 rounded-lg bg-destructive/5 border border-destructive/20 text-sm">
    <span className="font-medium text-destructive">Причина отмены:</span>{" "}
    <span className="text-foreground/90">{event.cancel_reason}</span>
  </div>
)}

            {/* ЗАЯВКИ ОРГАНИЗАТОРА */}
            {isOrganizer && (
              <div id="applications" className="mt-6 scroll-mt-24">
                <EventApplicationsList
                  eventId={event.id}
                  accessToken={accessToken}
                />
              </div>
            )}

            {/* ЧАТ */}
            {isCancelled ? (
              <div className="mt-6 p-6 border rounded-xl bg-muted/20 text-center text-sm text-muted-foreground">
                Событие отменено — чат закрыт.
              </div>
            ) : isCompleted ? (
              <div className="mt-6 p-6 border rounded-xl bg-muted/20 text-center text-sm text-muted-foreground">
                Событие завершено — чат закрыт.
              </div>
            ) : canChat ? (
              <div className="mt-6">
                <EventChat
                  eventId={event.id}
                  token={accessToken}
                  currentUserId={userId}
                />
              </div>
            ) : (
              <div className="mt-6 p-6 border rounded-xl bg-muted/20 text-center text-sm text-muted-foreground">
                Чат доступен участникам, подавшим заявку, и организатору.
              </div>
            )}
          </div>
        </div>

        {/* ПРАВАЯ КОЛОНКА: Описание + Карта + Участники */}
        <div className="flex flex-col gap-6">
          <div className="border rounded-xl p-6 bg-card">
            <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-4">
              Описание
            </h2>
            <p className="text-sm leading-relaxed mb-6 whitespace-pre-wrap">
              {event.description || "Описание не добавлено."}
            </p>

            <div className="flex items-start gap-2 text-sm text-muted-foreground mb-4">
              <MapPin className="h-4 w-4 shrink-0 mt-0.5" />
              <span>
                {event.city}
                {canSeeExact && event.address ? `, ${event.address}` : ""}
              </span>
            </div>

                <EventMap
                  lat={event.lat}
                  lng={event.lng}
                  city={event.city}
                  hideExactLocation={event.hide_exact_location}
                  canSeeExact={canSeeExact ?? false}
                  heightClass="h-80"
                />
          </div>

          {!isCancelled && (
            <ParticipantsList eventId={event.id} />
          )}
        </div>
      </div>
            <CancelDialog
        open={cancelEventDialogOpen}
        onOpenChange={setCancelEventDialogOpen}
        variant="event"
        onConfirm={handleCancelEvent}
        loading={cancellingEvent}
      />

      <CancelDialog
        open={cancelAppDialogOpen}
        onOpenChange={setCancelAppDialogOpen}
        variant="application"
        onConfirm={handleCancelApplication}
        loading={cancellingApp}
      />
    </div>
  );
}