"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Star, User } from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { apiFetch } from "@/lib/api";
import { avatarUrl } from "@/lib/avatars";
import { useAuth } from "@/lib/auth-context";

type Participant = {
  id: string;
  username: string;
  avatar_url: string | null;
  city: string | null;
  age?: number | null;
  is_organizer: boolean;
};

const INITIAL_LIMIT = 8;

export function ParticipantsList({ eventId }: { eventId: string }) {
  const { accessToken, isReady } = useAuth();
  const [participants, setParticipants] = useState<Participant[]>([]);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState(false);

  useEffect(() => {
    if (!isReady) return;
    let cancelled = false;
    apiFetch(`/events/${eventId}/participants`, {}, accessToken)
      .then((res) => res.json())
      .then((data: { participants?: Participant[] }) => {
        if (!cancelled) setParticipants(data.participants ?? []);
      })
      .catch(() => {
        // silent — не ломаем страницу события из-за сайдбара
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [eventId, accessToken, isReady]);

  if (loading) {
    return (
      <div className="border rounded-xl p-6 bg-card">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-4">
          Кто идёт
        </h2>
        <div className="grid grid-cols-4 gap-3">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="aspect-square rounded-full" />
          ))}
        </div>
      </div>
    );
  }

  if (participants.length === 0) {
    return (
      <div className="border rounded-xl p-6 bg-card">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-2">
          Кто идёт
        </h2>
        <p className="text-sm text-muted-foreground">
          Пока никто не присоединился.
        </p>
      </div>
    );
  }

  const visible = expanded
    ? participants
    : participants.slice(0, INITIAL_LIMIT);
  const hasMore = participants.length > INITIAL_LIMIT;

  return (
    <div className="border rounded-xl p-6 bg-card">
      <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-4">
        Кто идёт ({participants.length})
      </h2>

      <div className="grid grid-cols-4 gap-3">
        {visible.map((p) => (
          <Link
            key={p.id}
            href={`/users/${p.id}`}
            className="group flex flex-col items-center gap-1.5"
            title={p.username}
          >
            <div className="relative">
              <Avatar className="h-12 w-12 ring-2 ring-transparent group-hover:ring-primary/50 transition-all">
                <AvatarImage
                  src={avatarUrl(p.avatar_url) ?? ""}
                  alt={p.username}
                />
                <AvatarFallback className="bg-muted text-xs">
                  {p.username?.[0]?.toUpperCase() ?? (
                    <User className="h-4 w-4" />
                  )}
                </AvatarFallback>
              </Avatar>
              {p.is_organizer && (
                <span
                  className="absolute -top-1 -right-1 h-4 w-4 rounded-full bg-primary text-primary-foreground flex items-center justify-center"
                  title="Организатор"
                >
                  <Star className="h-2.5 w-2.5 fill-current" />
                </span>
              )}
            </div>
            <span className="text-xs text-muted-foreground truncate w-full text-center">
              {p.username}
            </span>
          </Link>
        ))}
      </div>

      {hasMore && (
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setExpanded((v) => !v)}
          className="w-full mt-3 text-xs"
        >
          {expanded ? "Свернуть" : `Показать всех (${participants.length})`}
        </Button>
      )}
    </div>
  );
}