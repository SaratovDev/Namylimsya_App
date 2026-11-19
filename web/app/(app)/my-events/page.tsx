"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Skeleton } from "@/components/ui/skeleton";
import { EventCard } from "@/components/event-card";
import { ApplicationCard } from "@/components/application-card";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { Event, ApplicationWithEvent } from "@/lib/types";
import { CancelDialog } from "@/components/cancel-dialog";

export default function MyEventsPage() {
  const { accessToken, isReady } = useAuth();

  const [organized, setOrganized] = useState<Event[]>([]);
  const [applications, setApplications] = useState<ApplicationWithEvent[]>([]);
  const [loadingOrg, setLoadingOrg] = useState(true);
  const [loadingApps, setLoadingApps] = useState(true);
  const [cancellingId, setCancellingId] = useState<string | null>(null);
  const [cancelTarget, setCancelTarget] = useState<string | null>(null);

  const loadOrganized = useCallback(async () => {
    if (!accessToken) return;
    setLoadingOrg(true);
    try {
      const res = await apiFetch("/users/me/events", {}, accessToken);
      const data = await res.json();
      setOrganized(data.events ?? []);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Не удалось загрузить события"
      );
    } finally {
      setLoadingOrg(false);
    }
  }, [accessToken]);

  const loadApplications = useCallback(async () => {
    if (!accessToken) return;
    setLoadingApps(true);
    try {
      const res = await apiFetch("/users/me/applications", {}, accessToken);
      const data = await res.json();
      setApplications(data.applications ?? []);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Не удалось загрузить заявки"
      );
    } finally {
      setLoadingApps(false);
    }
  }, [accessToken]);

  useEffect(() => {
    if (!isReady || !accessToken) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-on-mount: setState происходит после await (ложное срабатывание)
    void loadOrganized();
    void loadApplications();
  }, [isReady, accessToken, loadOrganized, loadApplications]);

  const handleCancelApplication = async (reason: string) => {
    if (!accessToken || !cancelTarget) return;

    setCancellingId(cancelTarget);
    try {
      await apiFetch(
        `/applications/${cancelTarget}/cancel`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ reason }),
        },
        accessToken
      );
      setApplications((prev) =>
        prev.map((a) =>
          a.id === cancelTarget
            ? { ...a, status: "cancelled" as const }
            : a
        )
      );
      toast.success("Заявка отменена");
      setCancelTarget(null);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Не удалось отменить заявку"
      );
    } finally {
      setCancellingId(null);
    }
  };

  if (!isReady) {
    return (
      <div className="max-w-3xl mx-auto py-6 px-4 space-y-3">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-48 w-full rounded-xl" />
      </div>
    );
  }

  if (!accessToken) {
    return (
      <div className="max-w-3xl mx-auto py-6 px-4">
        <h1 className="text-3xl font-bold mb-6">Мои события</h1>
        <p className="text-muted-foreground">
          Войдите, чтобы видеть свои события и заявки.
        </p>
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto py-6 px-4">
      <h1 className="text-3xl font-bold mb-6">Мои события</h1>

      <Tabs defaultValue="organized">
        <TabsList className="mb-6">
          <TabsTrigger value="organized">
            Организую
            {!loadingOrg && organized.length > 0 && (
              <span className="ml-1.5 text-xs text-muted-foreground">
                ({organized.length})
              </span>
            )}
          </TabsTrigger>
          <TabsTrigger value="applications">
            Участвую
            {!loadingApps && applications.length > 0 && (
              <span className="ml-1.5 text-xs text-muted-foreground">
                ({applications.length})
              </span>
            )}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="organized">
          {loadingOrg ? (
            <div className="space-y-3">
              <Skeleton className="h-56 w-full rounded-xl" />
              <Skeleton className="h-56 w-full rounded-xl" />
            </div>
          ) : organized.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              <p>Вы ещё не создали ни одного события.</p>
              <p className="text-sm mt-2">
                Нажмите «Создать событие» в шапке, чтобы начать.
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              {organized.map((e) => (
                <EventCard
                  key={e.id}
                  event={e}
                  onRefresh={loadOrganized}
                />
              ))}
            </div>
          )}
        </TabsContent>

        <TabsContent value="applications">
          {loadingApps ? (
            <div className="space-y-3">
              <Skeleton className="h-40 w-full rounded-xl" />
              <Skeleton className="h-40 w-full rounded-xl" />
            </div>
          ) : applications.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              <p>Вы ещё не подали ни одной заявки.</p>
              <p className="text-sm mt-2">
                Найдите интересное событие в ленте и нажмите «Хочу участвовать».
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {applications.map((a) => (
                <ApplicationCard
                  key={a.id}
                  application={a}
                  onCancel={(id) => setCancelTarget(id)}                  cancelling={cancellingId === a.id}
                />
              ))}
            </div>
          )}
        </TabsContent>
      </Tabs>
            <CancelDialog
        open={cancelTarget !== null}
        onOpenChange={(open) => {
          if (!open) setCancelTarget(null);
        }}
        variant="application"
        onConfirm={handleCancelApplication}
        loading={cancellingId !== null}
      />
    </div>
  );
}