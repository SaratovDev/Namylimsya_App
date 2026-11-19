"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { Check, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { apiFetch } from "@/lib/api";
import { toast } from "sonner";

type App = {
  id: string;
  user_id: string;
  username: string;
  avatar_url: string | null;
  status: "pending" | "approved" | "rejected" | "cancelled";
  created_at: string;
};

type Props = {
  eventId: string;
  accessToken: string | null;
};

export function EventApplicationsList({ eventId, accessToken }: Props) {
  const [apps, setApps] = useState<App[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchApps = async () => {
    try {
      const res = await apiFetch(`/events/${eventId}/applications`, {}, accessToken);
      const data = await res.json();
      setApps(data.applications || []);
    } catch {
      toast.error("Не удалось загрузить заявки");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchApps();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [eventId]);

  const approve = async (id: string) => {
    try {
      await apiFetch(`/applications/${id}/approve`, { method: "PATCH" }, accessToken);
      toast.success("Заявка одобрена");
      fetchApps();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Ошибка");
    }
  };

  const reject = async (id: string) => {
    try {
      await apiFetch(`/applications/${id}/reject`, { method: "PATCH" }, accessToken);
      toast.success("Заявка отклонена");
      fetchApps();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Ошибка");
    }
  };

  if (loading) return <div className="text-muted-foreground">Загрузка заявок...</div>;
  if (apps.length === 0) return null;

  const pending = apps.filter((a) => a.status === "pending");
  const approved = apps.filter((a) => a.status === "approved");

  return (
    <div>
      <h2 className="text-xl font-semibold mb-3">
        Заявки на участие ({pending.length} новых)
      </h2>

      {pending.length > 0 && (
        <div className="space-y-2 mb-6">
          <p className="text-sm text-muted-foreground mb-2">Ожидают рассмотрения:</p>
          {pending.map((app) => (
            <div key={app.id} className="flex items-center gap-3 p-3 border rounded-lg">
              <Link
                href={`/users/${app.user_id}`}
                className="flex items-center gap-3 flex-1 min-w-0 group/app"
              >
                <Avatar className="h-10 w-10">
                  <AvatarImage src={app.avatar_url ?? ""} />
                  <AvatarFallback className="bg-secondary">
                    {app.username.slice(0, 1).toUpperCase()}
                  </AvatarFallback>
                </Avatar>
                <span className="font-medium truncate group-hover/app:text-primary transition-colors">
                  {app.username}
                </span>
              </Link>
              <Button size="sm" onClick={() => approve(app.id)} title="Одобрить">
                <Check className="h-4 w-4" />
              </Button>
              <Button size="sm" variant="outline" onClick={() => reject(app.id)} title="Отклонить">
                <X className="h-4 w-4" />
              </Button>
            </div>
          ))}
        </div>
      )}

      {approved.length > 0 && (
        <div className="space-y-2">
          <p className="text-sm text-muted-foreground mb-2">Одобрены ({approved.length}):</p>
          {approved.map((app) => (
            <div key={app.id} className="flex items-center gap-3 p-3 border rounded-lg bg-muted/30">
              <Link
                href={`/users/${app.user_id}`}
                className="flex items-center gap-3 flex-1 min-w-0 group/app"
              >
                <Avatar className="h-8 w-8">
                  <AvatarImage src={app.avatar_url ?? ""} />
                  <AvatarFallback className="bg-secondary text-xs">
                    {app.username.slice(0, 1).toUpperCase()}
                  </AvatarFallback>
                </Avatar>
                <span className="text-sm truncate group-hover/app:text-primary transition-colors">
                  {app.username}
                </span>
              </Link>
              <Badge variant="secondary" className="text-xs">Одобрено</Badge>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}