"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Calendar,
  CalendarDays,
  Bell,
  User,
  Settings,
  LifeBuoy,
  Shield,
  ChevronLeft,
  Menu,
  Moon,
  Sun,
  LogOut,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { useTheme } from "next-themes";
import { useEffect, useState } from "react";
import { useAuth } from "@/lib/auth-context";
import { decodeJwt } from "@/lib/jwt";

const navItems = [
  { href: "/", label: "Лента", icon: CalendarDays },
  { href: "/my-events", label: "Мои события", icon: Calendar },
  { href: "/notifications", label: "Уведомления", icon: Bell },
  { href: "/profile", label: "Профиль", icon: User },
  { href: "/settings", label: "Настройки", icon: Settings },
  { href: "/support", label: "Поддержка", icon: LifeBuoy },
];

type AppSidebarProps = {
  onNavigate?: () => void;
  isMobile?: boolean;
};

export function AppSidebar({ onNavigate, isMobile = false }: AppSidebarProps = {}) {
  const pathname = usePathname();
  const [collapsed, setCollapsed] = useState(false);
  const { theme, setTheme } = useTheme();
  const { accessToken, logout } = useAuth();

  const role = accessToken ? decodeJwt(accessToken)?.role : null;
  const canSeeAdmin = role === "admin" || role === "moderator";

  const [mounted, setMounted] = useState(false);
useEffect(() => {
  queueMicrotask(() => setMounted(true));
}, []);

  return (
    <aside
      className={cn(
        "bg-card flex flex-col transition-[width] duration-300",
        !isMobile && "h-screen border-r border-border md:sticky md:top-0", 
        isMobile && "h-full w-full",
        !isMobile && (collapsed ? "w-[72px]" : "w-[260px]")
      )}
    >

      {/* Header */}
      <div className="flex items-center h-16 px-4 border-b border-border">
        {!collapsed && (
          <Link href="/" className="font-bold text-xl tracking-tight">
            Намылимся<span className="text-primary">.</span>
          </Link>
        )}
        {!isMobile && (
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setCollapsed(!collapsed)}
            className={cn("ml-auto", collapsed && "mx-auto")}
            aria-label={collapsed ? "Развернуть меню" : "Свернуть меню"}
          >
            {collapsed ? <Menu className="h-5 w-5" /> : <ChevronLeft className="h-5 w-5" />}
          </Button>
        )}
      </div>

      {/* Nav */}
      <nav className="flex-1 py-3 px-2 space-y-1 overflow-y-auto">
        {navItems.map((item) => {
          const active = pathname === item.href || (item.href !== "/" && pathname.startsWith(item.href));
          const Icon = item.icon;
          return (
            <Link
              key={item.href}
              href={item.href}
              onClick={onNavigate}
              className={cn(
                "flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors",
                active
                  ? "bg-primary/15 text-primary font-semibold"
                  : "text-muted-foreground hover:bg-muted/50 hover:text-foreground",                collapsed && "justify-center px-0"
              )}
              title={collapsed ? item.label : undefined}
            >
              <Icon className="h-5 w-5 shrink-0" />
              {!collapsed && <span>{item.label}</span>}
            </Link>
          );
        })}

        {/* Admin link — только если role=admin (пока хардкод, в 10.3 сделаем проверку) */}
         {canSeeAdmin && (
          <Link
            href="/admin"
            onClick={onNavigate}
            className={cn(
              "flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors",
              pathname.startsWith("/admin")
                ? "bg-destructive/10 text-destructive font-medium"
                : "text-muted-foreground hover:bg-destructive/10 hover:text-destructive",
              collapsed && "justify-center px-0"
            )}
            title={collapsed ? "Админка" : undefined}
          >
            <Shield className="h-5 w-5 shrink-0" />
            {!collapsed && <span>Админка</span>}
          </Link>
        )}
      </nav>

      {/* Footer */}
      <div className="border-t border-border p-2 space-y-1">
        <Button
          variant="ghost"
          className={cn("w-full justify-start gap-3", collapsed && "justify-center px-0")}
          onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
        >
        {mounted ? (
          theme === "dark" ? <Sun className="h-5 w-5" /> : <Moon className="h-5 w-5" />
        ) : (
          <Sun className="h-5 w-5" />  // заглушка до монтирования — сервер и клиент совпадут
        )}
          {!collapsed && <span>Тема</span>}
        </Button>
        {accessToken && (
          <Button
            variant="ghost"
            onClick={() => {
              onNavigate?.();
              void logout();
            }}
            className={cn(
              "w-full justify-start gap-3 text-muted-foreground",
              collapsed && "justify-center px-0"
            )}
          >
            <LogOut className="h-5 w-5" />
            {!collapsed && <span>Выйти</span>}
          </Button>
        )}
      </div>
    </aside>
  );
}