"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  Users,
  CalendarDays,
  Flag,
  LifeBuoy,
  ScrollText,
  Home,
  LogOut,
} from "lucide-react";
import type { ComponentType } from "react";
import { useAuth } from "@/lib/auth-context";
import { decodeJwt } from "@/lib/jwt";
import { cn } from "@/lib/utils";

type Role = "admin" | "moderator" | "user";

const NAV_ITEMS: Array<{
  href: string;
  label: string;
  icon: ComponentType<{ className?: string }>;
  roles: Role[];
}> = [
  { href: "/admin", label: "Дашборд", icon: LayoutDashboard, roles: ["admin"] },
  { href: "/admin/users", label: "Пользователи", icon: Users, roles: ["admin", "moderator"] },
  { href: "/admin/events", label: "События", icon: CalendarDays, roles: ["admin", "moderator"] },
  { href: "/admin/complaints", label: "Жалобы", icon: Flag, roles: ["admin", "moderator"] },
  { href: "/admin/support", label: "Поддержка", icon: LifeBuoy, roles: ["admin", "moderator"] },
  { href: "/admin/audit-log", label: "Аудит", icon: ScrollText, roles: ["admin"] },
];

export function AdminNav() {
  const pathname = usePathname();
  const { accessToken, logout } = useAuth();

  const role: Role | null = accessToken
    ? ((decodeJwt(accessToken)?.role as Role | undefined) ?? "user")
    : null;

  const canSeeNav = role === "admin" || role === "moderator";
  const visibleNav = canSeeNav
    ? NAV_ITEMS.filter((item) => item.roles.includes(role as Role))
    : [];

  return (
    <>
      <nav className="flex items-center gap-1 text-sm overflow-x-auto min-w-0">
        {visibleNav.map((item) => {
          const Icon = item.icon;
          const isActive =
            pathname === item.href ||
            (item.href !== "/admin" && pathname.startsWith(item.href + "/"));
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-1.5 px-2.5 py-1.5 rounded-md whitespace-nowrap transition-colors",
                isActive
                  ? "bg-brand-cream/10 text-primary font-medium"
                  : "hover:bg-brand-cream/5 hover:text-primary/90"
              )}
            >
              <Icon className="h-4 w-4" />
              <span className="hidden sm:inline">{item.label}</span>
            </Link>
          );
        })}
      </nav>

      <div className="ml-auto flex items-center gap-1 shrink-0">
        {role && (
          <span className="text-xs opacity-70 hidden md:inline pr-2">
            {role === "admin" ? "админ" : role === "moderator" ? "модератор" : ""}
          </span>
        )}
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 px-2 py-1.5 rounded-md text-xs hover:bg-brand-cream/10 transition-colors"
          title="На сайт"
        >
          <Home className="h-4 w-4" />
          <span className="hidden sm:inline">На сайт</span>
        </Link>
        {accessToken && (
          <button
            type="button"
            onClick={() => {
              void logout();
            }}
            className="inline-flex items-center gap-1.5 px-2 py-1.5 rounded-md text-xs hover:bg-brand-cream/10 transition-colors"
            title="Выйти"
          >
            <LogOut className="h-4 w-4" />
            <span className="hidden sm:inline">Выйти</span>
          </button>
        )}
      </div>
    </>
  );
}

export function AdminNavSkeleton() {
  return (
    <>
      <div className="flex items-center gap-1">
        {[1, 2, 3, 4].map((i) => (
          <div key={i} className="h-7 w-20 rounded-md bg-brand-cream/10 animate-pulse" />
        ))}
      </div>
      <div className="ml-auto h-7 w-32 rounded-md bg-brand-cream/10 animate-pulse" />
    </>
  );
}