"use client";

import Link from "next/link";
import { ShieldAlert, Loader2 } from "lucide-react";
import type { ComponentType } from "react";
import { useAuth } from "@/lib/auth-context";
import { decodeJwt } from "@/lib/jwt";
import { buttonVariants } from "@/components/ui/button";

type Role = "admin" | "moderator" | "user";

type Props = {
  children: React.ReactNode;
  /** Минимальные роли, которым доступна страница. */
  allow: Role[];
};

function FullscreenMessage({
  icon: Icon,
  title,
  description,
  action,
}: {
  icon: ComponentType<{ className?: string }>;
  title: string;
  description: string;
  action: { href: string; label: string };
}) {
  return (
    <div className="flex flex-col items-center justify-center py-24 text-center px-4">
      <Icon className="h-10 w-10 text-muted-foreground mb-3" />
      <h1 className="text-xl font-semibold mb-2">{title}</h1>
      <p className="text-sm text-muted-foreground mb-4 max-w-md">
        {description}
      </p>
      <Link href={action.href} className={buttonVariants()}>
        {action.label}
      </Link>
    </div>
  );
}

export function AdminGuard({ children, allow }: Props) {
  const { accessToken, isReady } = useAuth();

  const role: Role | null = accessToken
    ? ((decodeJwt(accessToken)?.role as Role | undefined) ?? "user")
    : null;

  if (!isReady) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!accessToken || !role) {
    return (
      <FullscreenMessage
        icon={ShieldAlert}
        title="Нужна авторизация"
        description="Войдите как администратор или модератор, чтобы открыть раздел."
        action={{ href: "/login", label: "Войти" }}
      />
    );
  }

  if (role !== "admin" && role !== "moderator") {
    return (
      <FullscreenMessage
        icon={ShieldAlert}
        title="Доступ запрещён"
        description="Раздел доступен только администраторам и модераторам."
        action={{ href: "/", label: "На главную" }}
      />
    );
  }

  if (!allow.includes(role)) {
    return (
      <FullscreenMessage
        icon={ShieldAlert}
        title="Недостаточно прав"
        description="Эта страница доступна только администраторам."
        action={{ href: "/admin", label: "К дашборду" }}
      />
    );
  }

  return <>{children}</>;
}