"use client";

import { useRouter } from "next/navigation";
import { Calendar, MapPin, User, X, MessageCircle } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { ApplicationWithEvent } from "@/lib/types";
import { formatEventDate } from "@/lib/event-utils";
import { avatarUrl } from "@/lib/avatars";
import { cn } from "@/lib/utils";
import Link from "next/link";

type Props = {
  application: ApplicationWithEvent;
  onCancel: (applicationId: string) => void;
  cancelling?: boolean;
};

export function ApplicationCard({
  application,
  onCancel,
  cancelling = false,
}: Props) {
  const router = useRouter();

  const isEventInactive =
    application.event_status === "cancelled" ||
    application.event_status === "completed";

  const canChat =
    !isEventInactive &&
    (application.status === "pending" || application.status === "approved");

  const canCancel =
    !isEventInactive &&
    (application.status === "pending" || application.status === "approved");

  const statusBadge = getStatusBadge(application.status);

  return (
    <Card
      className={cn(
        "p-4 sm:p-6 transition-shadow",
        !isEventInactive && "hover:shadow-md cursor-pointer",
        isEventInactive && "opacity-75"
      )}
      onClick={() => {
        if (!isEventInactive) {
          router.push(`/events/${application.event_id}`);
        }
      }}
    >
      <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap mb-2">
            <Badge variant={statusBadge.variant} className={statusBadge.className}>
              {statusBadge.text}
            </Badge>
            {application.event_status === "cancelled" && (
              <Badge variant="destructive">Событие отменено</Badge>
            )}
            {application.event_status === "completed" && (
              <Badge variant="secondary">Событие завершено</Badge>
            )}
          </div>

          {application.event_status === "cancelled" &&
            application.event_cancel_reason && (
              <p className="text-xs text-destructive/80 mb-2">
                Причина отмены: {application.event_cancel_reason}
              </p>
            )}

          <h3 className="text-lg font-semibold mb-3 truncate">
            {application.event_title}
          </h3>

          <div className="flex flex-col gap-1.5 text-sm text-muted-foreground">
            <div className="flex items-center gap-1.5">
              <Calendar className="h-3.5 w-3.5 shrink-0" />
              <span>{formatEventDate(application.event_starts_at)}</span>
            </div>
            <div className="flex items-center gap-1.5">
              <MapPin className="h-3.5 w-3.5 shrink-0" />
              <span>{application.event_city}</span>
            </div>
            <Link
              href={`/users/${application.event_organizer_id}`}
              className="flex items-center gap-2 mt-2 group/org w-fit"
              onClick={(e) => e.stopPropagation()}
            >
              <Avatar className="h-6 w-6">
                <AvatarImage
                  src={avatarUrl(application.organizer_avatar) ?? ""}
                  alt={application.organizer_name}
                />
                <AvatarFallback className="bg-muted">
                  <User className="h-3 w-3" />
                </AvatarFallback>
              </Avatar>
              <span className="text-xs group-hover/org:text-primary transition-colors">
                {application.organizer_name}
              </span>
            </Link>
        </div> </div>

        {(canChat || canCancel) && (
          <div
            className="flex flex-row sm:flex-col gap-2 shrink-0"
            onClick={(e) => e.stopPropagation()}
          >
            {canChat && (
              <Button
                size="sm"
                variant="outline"
                onClick={(e) => {
                  e.stopPropagation();
                  router.push(`/events/${application.event_id}`);
                }}
              >
                <MessageCircle className="h-4 w-4 mr-1" />
                В чат
              </Button>
            )}
            {canCancel && (
              <Button
                size="sm"
                variant="ghost"
                className="text-destructive hover:text-destructive"
                onClick={(e) => {
                  e.stopPropagation();
                  onCancel(application.id);
                }}
                disabled={cancelling}
              >
                <X className="h-4 w-4 mr-1" />
                {cancelling ? "Отмена..." : "Отменить"}
              </Button>
            )}
          </div>
        )}
      </div>
    </Card>
  );
}

function getStatusBadge(status: ApplicationWithEvent["status"]): {
  text: string;
  variant: "default" | "secondary" | "destructive" | "outline";
  className?: string;
} {
  switch (status) {
    case "pending":
      return { text: "Ожидает одобрения", variant: "secondary" };
    case "approved":
      return {
        text: "Вы приняты",
        variant: "outline",
        className:
          "bg-green-600/10 text-green-600 border-green-600/30 dark:text-green-400",
      };
    case "rejected":
      return { text: "Отклонено", variant: "destructive" };
    case "cancelled":
      return { text: "Отменено", variant: "outline" };
    default:
      return { text: status, variant: "outline" };
  }
}