"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  AlertCircle,
  Calendar,
  ChevronLeft,
  Flag,
  MapPin,
  RefreshCw,
  Trophy,
  Users,
  Ticket,
} from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { ReportUserDialog } from "@/components/report-user-dialog";
import { apiFetch } from "@/lib/api";
import { avatarUrl } from "@/lib/avatars";
import { useAuth } from "@/lib/auth-context";
import { categoryLabel } from "@/lib/categories";
import { decodeJwt } from "@/lib/jwt";
import { pluralRu } from "@/lib/plural";
import { ProfileEvent, PublicProfile } from "@/lib/types";
import { cn } from "@/lib/utils";

const INITIAL_LIMIT = 10;
const EXPANDED_LIMIT = 50;

function formatDaysOnPlatform(n: number): string {
  if (n === 0) return "Намыливается только что";
  return `Намыливается ${n} ${pluralRu(n, "день", "дня", "дней")}`;
}

function formatShortDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString("ru-RU", {
    day: "numeric",
    month: "short",
  });
}

export default function UserProfilePage() {
  const params = useParams();
  const router = useRouter();
  const userId = params.id as string;
  const { accessToken, isReady } = useAuth();
  const [myHobbies, setMyHobbies] = useState<string[]>([]);


  const [profile, setProfile] = useState<PublicProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reportOpen, setReportOpen] = useState(false);
  const [expandedOrg, setExpandedOrg] = useState(false);
  const [expandedAtt, setExpandedAtt] = useState(false);
  const [expandingOrg, setExpandingOrg] = useState(false);
  const [expandingAtt, setExpandingAtt] = useState(false);

  const loadMyHobbies = useCallback(async () => {
    if (!isReady || !accessToken) return;
    try {
      const res = await apiFetch("/users/me", {}, accessToken);
      const data = await res.json();
      setMyHobbies(data.hobbies ?? []);
    } catch {
      // silent
    }
  }, [accessToken, isReady]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-on-mount: setState происходит после await (ложное срабатывание)
    void loadMyHobbies();
  }, [loadMyHobbies]);

  const load = useCallback(

    
    async (limit: number = INITIAL_LIMIT) => {
      if (!isReady) return;
      try {
        const res = await apiFetch(`/users/${userId}?limit=${limit}`, {}, accessToken);
        const data = (await res.json()) as PublicProfile;
        setProfile(data);
        setError(null);
      } catch (err) {
        const msg = err instanceof Error ? err.message : "";
        if (msg.toLowerCase().includes("not found") || msg.includes("404")) {
          setError("Профиль не найден");
        } else {
          setError("Не удалось загрузить профиль");
        }
      } finally {
        setLoading(false);
      }
    },
    [userId, accessToken, isReady]
  );

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const expandOrganized = async () => {
    setExpandingOrg(true);
    try {
      await load(EXPANDED_LIMIT);
      setExpandedOrg(true);
    } finally {
      setExpandingOrg(false);
    }
  };

  const expandAttended = async () => {
    setExpandingAtt(true);
    try {
      await load(EXPANDED_LIMIT);
      setExpandedAtt(true);
    } finally {
      setExpandingAtt(false);
    }
  };

  // ─── Loading / Error ──────────────────────────────────────────────

  if (loading) {
    return (
      <div className="max-w-3xl mx-auto py-6 px-4 space-y-4">
        <Skeleton className="h-8 w-24" />
        <Skeleton className="h-48 w-full rounded-xl" />
        <Skeleton className="h-32 w-full rounded-xl" />
      </div>
    );
  }

  if (error || !profile) {
    return (
      <div className="max-w-3xl mx-auto py-6 px-4 flex flex-col items-center py-16 text-center">
        <AlertCircle className="h-10 w-10 text-destructive mb-3" />
        <p className="text-sm text-muted-foreground mb-4">
          {error ?? "Профиль не найден"}
        </p>
        <div className="flex gap-2">
          <Button variant="ghost" onClick={() => router.back()}>
            Назад
          </Button>
          <Button
            variant="outline"
            onClick={() => void load()}
            className="gap-2"
          >
            <RefreshCw className="h-4 w-4" />
            Повторить
          </Button>
        </div>
      </div>
    );
  }

  // ─── Derived ──────────────────────────────────────────────────────

  const isSelf =
    accessToken && decodeJwt(accessToken)?.user_id === profile.id;
  const canReport = !!accessToken && !isSelf && !profile.is_banned;

  const organizedVisible = expandedOrg
    ? profile.organized_events
    : profile.organized_events.slice(0, INITIAL_LIMIT);
  const attendedVisible = expandedAtt
    ? profile.attended_events
    : profile.attended_events.slice(0, INITIAL_LIMIT);

  // ─── Render ───────────────────────────────────────────────────────

  return (
    <TooltipProvider>
      <div className="max-w-3xl mx-auto py-6 px-4">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => router.back()}
          className="mb-4 -ml-2 gap-1 text-muted-foreground"
        >
          <ChevronLeft className="h-4 w-4" />
          Назад
        </Button>

        {/* ─── Profile header ──────────────────────────────────────── */}
        <Card className="p-6 relative mb-4">
          <div className="absolute top-4 right-4 flex items-center gap-2">
            {profile.is_banned && (
              <Badge variant="destructive" className="text-xs font-bold">
                BANNED
              </Badge>
            )}
            {canReport && (
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      onClick={() => setReportOpen(true)}
                      aria-label="Пожаловаться на пользователя"
                      className="text-muted-foreground hover:text-destructive"
                    >
                      <Flag className="h-4 w-4" />
                    </Button>
                  }
                />
                <TooltipContent>Пожаловаться</TooltipContent>
              </Tooltip>
            )}
          </div>

          <div className="flex items-start gap-4 mb-4">
            <Avatar className="h-24 w-24 shrink-0">
              <AvatarImage
                src={avatarUrl(profile.avatar_url) ?? ""}
                alt={profile.username}
              />
              <AvatarFallback className="bg-primary text-primary-foreground text-3xl">
                {profile.username?.[0]?.toUpperCase() ?? "?"}
              </AvatarFallback>
            </Avatar>

            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 flex-wrap mb-1">
                <h1 className="text-2xl font-bold truncate">
                  @{profile.username}
                </h1>
                {profile.role === "admin" && (
                  <Badge className="text-xs">Администратор</Badge>
                )}
                {profile.role === "moderator" && (
                  <Badge variant="secondary" className="text-xs">
                    Модератор
                  </Badge>
                )}
              </div>

              <p className="text-sm font-medium text-primary mb-2">
                {profile.title}
              </p>

              <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-foreground">
                {profile.age != null && (
                  <span>
                    {profile.age} {pluralRu(profile.age, "год", "года", "лет")}
                  </span>
                )}
                {profile.city && (
                  <span className="inline-flex items-center gap-1">
                    <MapPin className="h-3.5 w-3.5" />
                    {profile.city}
                  </span>
                )}
                <span className="inline-flex items-center gap-1">
                  <Calendar className="h-3.5 w-3.5" />
                  {formatDaysOnPlatform(profile.days_on_platform)}
                </span>
              </div>
            </div>
          </div>

          {profile.bio && (
            <div className="pt-4 border-t">
              <h2 className="text-xs uppercase tracking-wider text-muted-foreground mb-2">
                О себе
              </h2>
              <p className="text-sm whitespace-pre-wrap">{profile.bio}</p>
            </div>
          )}

          {profile.hobbies && profile.hobbies.length > 0 && (
            <div className="mt-4 pt-4 border-t">
              <h2 className="text-xs uppercase tracking-wider text-muted-foreground mb-2">
                Интересы
              </h2>
              <div className="flex flex-wrap gap-1.5">
                {profile.hobbies.map((h) => {
const matchesMe = !isSelf && myHobbies.includes(h);
                  return (
                    <span
                      key={h}
                      className={cn(
                        "text-xs px-2 py-0.5 rounded-full",
                        matchesMe
                          ? "bg-primary/20 text-primary border border-primary/40 font-medium"
                          : "bg-muted text-muted-foreground"
                      )}
                    >
                      {matchesMe && "✓ "}
                      {h}
                    </span>
                  );
                })}
              </div>
 {!isSelf && myHobbies.length > 0 && profile.hobbies.some((h) => myHobbies.includes(h)) && (
                  <p className="text-xs text-primary mt-2">
                    Совпадает с вашими интересами:{" "}
                    {profile.hobbies
                      .filter((h) => myHobbies.includes(h))
                      .length}
                  </p>
                )}
            </div>
          )}
        </Card>

        {/* ─── Badges (stubs) ─────────────────────────────────────── */}
        <Card className="p-6 mb-4">
          <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-1">
            Награды
          </h2>
          <p className="text-xs text-muted-foreground mb-4">
            Система наград в разработке — скоро здесь появятся достижения.
          </p>
          <div className="grid grid-cols-2 gap-3">
            <div className="border rounded-lg p-3 opacity-50 flex flex-col items-center text-center gap-1">
              <Trophy className="h-6 w-6 text-muted-foreground" />
              <p className="text-sm font-medium">Организатор</p>
              <p className="text-[10px] text-muted-foreground">
                Проведи 5 событий
              </p>
            </div>
            <div className="border rounded-lg p-3 opacity-50 flex flex-col items-center text-center gap-1">
              <Ticket className="h-6 w-6 text-muted-foreground" />
              <p className="text-sm font-medium">Гость</p>
              <p className="text-[10px] text-muted-foreground">
                Посети 10 событий
              </p>
            </div>
          </div>
        </Card>

        {/* ─── Organized ──────────────────────────────────────────── */}
        <Card className="p-6 mb-4">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground">
              Организовал ({profile.organized_count})
            </h2>
            {profile.organized_count > INITIAL_LIMIT && !expandedOrg && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => void expandOrganized()}
                disabled={expandingOrg}
              >
                {expandingOrg
                  ? "Загрузка…"
                  : `Показать все (${profile.organized_count})`}
              </Button>
            )}
          </div>
          {profile.organized_events.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              Пока нет завершённых событий.
            </p>
          ) : (
            <div className="space-y-2">
              {organizedVisible.map((e) => (
                <ProfileEventCard key={e.id} event={e} variant="organized" />
              ))}
            </div>
          )}
        </Card>

        {/* ─── Attended ───────────────────────────────────────────── */}
        <Card className="p-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground">
              Посетил ({profile.attended_count})
            </h2>
            {profile.attended_count > INITIAL_LIMIT && !expandedAtt && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => void expandAttended()}
                disabled={expandingAtt}
              >
                {expandingAtt
                  ? "Загрузка…"
                  : `Показать все (${profile.attended_count})`}
              </Button>
            )}
          </div>
          {profile.attended_events.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              Пока нет посещённых событий.
            </p>
          ) : (
            <div className="space-y-2">
              {attendedVisible.map((e) => (
                <ProfileEventCard key={e.id} event={e} variant="attended" />
              ))}
            </div>
          )}
        </Card>

        <ReportUserDialog
          open={reportOpen}
          onOpenChange={setReportOpen}
          userId={profile.id}
          username={profile.username}
        />
      </div>
    </TooltipProvider>
  );
}

// ─── ProfileEventCard ────────────────────────────────────────────────

function ProfileEventCard({
  event,
  variant,
}: {
  event: ProfileEvent;
  variant: "organized" | "attended";
}) {
  return (
    <Link
      href={`/events/${event.id}`}
      className="flex items-start gap-3 p-3 rounded-lg border hover:bg-muted/30 transition-colors"
    >
      <div className="text-center shrink-0 w-12">
        <div className="text-xs uppercase text-muted-foreground">
          {formatShortDate(event.starts_at)}
        </div>
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium truncate">{event.title}</p>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground mt-0.5">
          <span>{categoryLabel(event.category)}</span>
          <span className="inline-flex items-center gap-1">
            <MapPin className="h-3 w-3" />
            {event.city}
          </span>
          {variant === "organized" &&
            typeof event.participants_count === "number" && (
              <span className="inline-flex items-center gap-1">
                <Users className="h-3 w-3" />
                {event.participants_count}
              </span>
            )}
          {variant === "attended" && event.organizer_name && (
            <span className="truncate">
              орг. @{event.organizer_name}
            </span>
          )}
        </div>
      </div>
    </Link>
  );
}