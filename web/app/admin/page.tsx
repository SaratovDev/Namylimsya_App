"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Users,
  CalendarDays,
  FileText,
  TrendingUp,
  AlertCircle,
  Loader2,
  RefreshCw,
} from "lucide-react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { AdminGuard } from "@/components/admin-guard";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { decodeJwt } from "@/lib/jwt";
import { categoryLabel } from "@/lib/categories";
import { cn } from "@/lib/utils";

// ─── Types ─────────────────────────────────────────────────────────────

type Dashboard = {
  users: { total: number; new_7d: number; banned: number };
  events: { total: number; active: number; cancelled: number; upcoming: number };
  applications: {
    total: number;
    pending: number;
    approved: number;
    rejected: number;
    cancelled: number;
  };
  top_categories: Array<{ category: string; count: number }>;
  top_organizers: Array<{
    id: string;
    username: string;
    email: string;
    events_count: number;
  }>;
  activity: Array<{ day: string; registrations: number }>;
};

// ─── Page (обёртка с guard) ────────────────────────────────────────────

export default function Page() {
  return (
    <AdminGuard allow={["admin"]}>
      <DashboardContent />
    </AdminGuard>
  );
}

// ─── DashboardContent ──────────────────────────────────────────────────

function DashboardContent() {
  const { accessToken, isReady } = useAuth();
  const router = useRouter();
  const [data, setData] = useState<Dashboard | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  // Эффект загружает данные и при монтировании, и при изменении reloadKey.
  // fetchData объявлена ВНУТРИ — ESLint не считает её «внешней setState-функцией».
  useEffect(() => {
    if (!isReady) return;

    const role = accessToken ? decodeJwt(accessToken)?.role : null;
    if (role === "moderator") {
      router.replace("/admin/users");
      return;
    }

    if (!accessToken) return;

    let cancelled = false;

    (async () => {
      try {
        const res = await apiFetch("/admin/dashboard", {}, accessToken);
        const json = (await res.json()) as Dashboard;
        if (!cancelled) {
          setData(json);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          const message =
            err instanceof Error
              ? err.message
              : "Не удалось загрузить статистику";
          setError(
            message.includes("forbidden") || message.includes("403")
              ? "Дашборд доступен только администратору"
              : message
          );
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [isReady, accessToken, router, reloadKey]);

  const reload = useCallback(() => {
    setLoading(true);
    setError(null);
    setReloadKey((k) => k + 1);
  }, []);

  if (!isReady || (loading && !data)) {
    return <DashboardSkeleton />;
  }

  if (error || !data) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <AlertCircle className="h-10 w-10 text-destructive mb-3" />
        <p className="text-sm text-muted-foreground mb-4">
          {error ?? "Нет данных"}
        </p>
        <Button variant="outline" onClick={reload} className="gap-2">
          <RefreshCw className="h-4 w-4" />
          Повторить
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Дашборд</h1>
        <Button
          variant="ghost"
          size="sm"
          onClick={reload}
          disabled={loading}
          className="gap-2"
        >
          {loading ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <RefreshCw className="h-4 w-4" />
          )}
          Обновить
        </Button>
      </div>

      {/* ─── Карточки ─────────────────────────────────────────────── */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <StatCard
          icon={Users}
          label="Пользователи"
          value={data.users.total}
          href="/admin/users"
          stats={[
            { label: "новых за 7 дней", value: data.users.new_7d },
            {
              label: "забанено",
              value: data.users.banned,
              accent: data.users.banned > 0 ? "destructive" : undefined,
            },
          ]}
        />
        <StatCard
          icon={CalendarDays}
          label="События"
          value={data.events.total}
          href="/admin/events"
          stats={[
            { label: "активных", value: data.events.active, accent: "default" },
            { label: "предстоящих", value: data.events.upcoming },
            {
              label: "отменённых",
              value: data.events.cancelled,
              accent: data.events.cancelled > 0 ? "destructive" : undefined,
            },
          ]}
        />
        <StatCard
          icon={FileText}
          label="Заявки"
          value={data.applications.total}
          stats={[
            {
              label: "pending",
              value: data.applications.pending,
              accent: data.applications.pending > 0 ? "warning" : undefined,
            },
            {
              label: "approved",
              value: data.applications.approved,
              accent: "default",
            },
            { label: "rejected", value: data.applications.rejected },
            { label: "cancelled", value: data.applications.cancelled },
          ]}
        />
      </div>

      {/* ─── Графики ──────────────────────────────────────────────── */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Card className="p-4 md:p-6">
          <div className="flex items-center gap-2 mb-4">
            <TrendingUp className="h-4 w-4 text-muted-foreground" />
            <h2 className="text-sm font-semibold">Регистрации за 30 дней</h2>
          </div>
          <ActivityChart data={data.activity} />
        </Card>

        <Card className="p-4 md:p-6">
          <h2 className="text-sm font-semibold mb-4">Топ категорий</h2>
          <TopCategories data={data.top_categories} />
        </Card>
      </div>

      {/* ─── Топ-организаторы ─────────────────────────────────────── */}
      <Card className="p-4 md:p-6">
        <h2 className="text-sm font-semibold mb-4">Топ организаторов</h2>
        <TopOrganizers data={data.top_organizers} />
      </Card>
    </div>
  );
}

// ─── StatCard ──────────────────────────────────────────────────────────

type Accent = "default" | "destructive" | "warning";

function StatCard({
  icon: Icon,
  label,
  value,
  href,
  stats,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: number;
  href?: string;
  stats: Array<{ label: string; value: number; accent?: Accent }>;
}) {
  const content = (
    <Card
      className={cn(
        "p-4 md:p-6 h-full transition-shadow",
        href && "hover:shadow-md cursor-pointer"
      )}
    >
      <div className="flex items-center gap-2 text-muted-foreground mb-3">
        <Icon className="h-4 w-4" />
        <span className="text-xs uppercase tracking-wider font-medium">
          {label}
        </span>
      </div>
      <div className="text-3xl font-bold mb-4">{value}</div>
      <div className="space-y-1.5">
        {stats.map((s) => (
          <div key={s.label} className="flex justify-between text-xs">
            <span className="text-muted-foreground">{s.label}</span>
            <span className={accentClass(s.accent)}>{s.value}</span>
          </div>
        ))}
      </div>
    </Card>
  );

  return href ? <Link href={href}>{content}</Link> : content;
}

function accentClass(accent?: Accent): string {
  switch (accent) {
    case "destructive":
      return "text-destructive font-medium";
    case "warning":
      return "text-amber-600 dark:text-amber-400 font-medium";
    case "default":
      return "text-foreground font-medium";
    default:
      return "text-muted-foreground";
  }
}

// ─── ActivityChart (inline SVG) ────────────────────────────────────────

function ActivityChart({
  data,
}: {
  data: Array<{ day: string; registrations: number }>;
}) {
  if (data.length === 0) {
    return (
      <div className="h-40 flex items-center justify-center text-sm text-muted-foreground">
        Нет данных
      </div>
    );
  }

  const maxValue = Math.max(...data.map((d) => d.registrations), 1);
  const width = 600;
  const height = 120;
  const padding = { top: 8, right: 8, bottom: 20, left: 8 };
  const chartW = width - padding.left - padding.right;
  const chartH = height - padding.top - padding.bottom;

  const points = data.map((d, i) => {
    const x = padding.left + (i / Math.max(1, data.length - 1)) * chartW;
    const y = padding.top + chartH - (d.registrations / maxValue) * chartH;
    return { x, y, d };
  });

  const polyline = points
    .map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`)
    .join(" ");
  const area = `${padding.left},${padding.top + chartH} ${polyline} ${
    padding.left + chartW
  },${padding.top + chartH}`;

  return (
    <div className="space-y-2">
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className="w-full h-auto"
        role="img"
        aria-label={`Регистрации за ${data.length} дней, максимум ${maxValue}`}
      >
        {[0, 0.5, 1].map((t) => (
          <line
            key={t}
            x1={padding.left}
            x2={padding.left + chartW}
            y1={padding.top + chartH * t}
            y2={padding.top + chartH * t}
            stroke="currentColor"
            strokeOpacity={0.1}
            strokeWidth={1}
          />
        ))}
        <polygon points={area} fill="var(--primary)" fillOpacity={0.1} />
        <polyline
          points={polyline}
          fill="none"
          stroke="var(--primary)"
          strokeWidth={2}
          strokeLinejoin="round"
          strokeLinecap="round"
        />
        {points.map((p, i) => (
          <circle key={i} cx={p.x} cy={p.y} r={1.5} fill="var(--primary)">
            <title>
              {p.d.day}: {p.d.registrations}
            </title>
          </circle>
        ))}
      </svg>
      <div className="flex justify-between text-[10px] text-muted-foreground px-1">
        <span>{data[0]?.day}</span>
        <span>максимум: {maxValue}</span>
        <span>{data[data.length - 1]?.day}</span>
      </div>
    </div>
  );
}

// ─── TopCategories ─────────────────────────────────────────────────────

function TopCategories({
  data,
}: {
  data: Array<{ category: string; count: number }>;
}) {
  if (data.length === 0) {
    return (
      <div className="h-40 flex items-center justify-center text-sm text-muted-foreground">
        Нет данных
      </div>
    );
  }

  const max = Math.max(...data.map((d) => d.count), 1);

  return (
    <div className="space-y-3">
      {data.map((item) => (
        <div key={item.category}>
          <div className="flex justify-between text-xs mb-1">
            <span className="font-medium truncate pr-2">
              {categoryLabel(item.category)}
            </span>
            <span className="text-muted-foreground shrink-0">
              {item.count}
            </span>
          </div>
          <div className="h-2 rounded-full bg-muted overflow-hidden">
            <div
              className="h-full bg-primary rounded-full transition-all"
              style={{ width: `${(item.count / max) * 100}%` }}
            />
          </div>
        </div>
      ))}
    </div>
  );
}

// ─── TopOrganizers ─────────────────────────────────────────────────────

function TopOrganizers({
  data,
}: {
  data: Array<{
    id: string;
    username: string;
    email: string;
    events_count: number;
  }>;
}) {
  if (data.length === 0) {
    return (
      <div className="py-6 text-center text-sm text-muted-foreground">
        Нет данных
      </div>
    );
  }

  return (
    <div className="divide-y divide-border">
      {data.map((org, i) => (
        <Link
          key={org.id}
          href={`/admin/users/${org.id}`}
          className="flex items-center gap-3 py-2.5 first:pt-0 last:pb-0 hover:bg-muted/50 -mx-2 px-2 rounded transition-colors"
        >
          <span className="text-xs font-mono text-muted-foreground w-6 shrink-0">
            #{i + 1}
          </span>
          <div className="flex-1 min-w-0">
            <div className="text-sm font-medium truncate">{org.username}</div>
            <div className="text-xs text-muted-foreground truncate">
              {org.email}
            </div>
          </div>
          <span className="text-sm font-medium shrink-0">
            {org.events_count}{" "}
            <span className="text-xs text-muted-foreground">соб.</span>
          </span>
        </Link>
      ))}
    </div>
  );
}

// ─── Skeleton ──────────────────────────────────────────────────────────

function DashboardSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-8 w-40" />
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-44 w-full rounded-xl" />
        ))}
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Skeleton className="h-56 w-full rounded-xl" />
        <Skeleton className="h-56 w-full rounded-xl" />
      </div>
      <Skeleton className="h-64 w-full rounded-xl" />
    </div>
  );
}