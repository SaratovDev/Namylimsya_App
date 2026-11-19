"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Menu, Plus } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { useAuth } from "@/lib/auth-context";
import { NotificationBell } from "./notification-bell";
import { CreateEventDialog } from "./create-event-dialog";
import { AppSidebar } from "./app-sidebar";
import { apiFetch } from "@/lib/api";
import { avatarUrl } from "@/lib/avatars";
import { decodeJwt } from "@/lib/jwt";

export function AppHeader() {
  const router = useRouter();
  const { accessToken, logout } = useAuth();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [me, setMe] = useState<{ username: string; avatar_url: string | null } | null>(null);

  // Сброс профиля при выходе — корректировка состояния при рендере
  // (вместо синхронного setState в useEffect).
  const [prevToken, setPrevToken] = useState(accessToken);
  if (prevToken !== accessToken) {
    setPrevToken(accessToken);
    if (!accessToken) setMe(null);
  }

  useEffect(() => {
    if (!accessToken) {
      return;
    }
    let cancelled = false;
    apiFetch("/users/me", {}, accessToken)
      .then((res) => res.json())
      .then((data) => {
        if (!cancelled) setMe({ username: data.username, avatar_url: data.avatar_url });
      })
      .catch(() => {
        // не критично, оставим fallback
      });
    return () => {
      cancelled = true;
    };
  }, [accessToken]);

  return (
    <>
      <header className="sticky top-0 z-40 h-16 border-b border-border bg-background/80 backdrop-blur-md supports-backdrop-filter:bg-background/60">
        <div className="w-full max-w-[1400px] mx-auto px-4 md:px-12 h-full flex items-center gap-3">
          {/* Мобильный гамбургер */}
          <button
            type="button"
            onClick={() => setMobileMenuOpen(true)}
            className="md:hidden inline-flex h-10 w-10 items-center justify-center rounded-md hover:bg-accent/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            aria-label="Открыть меню"
          >
            <Menu className="h-5 w-5" />
          </button>

          {/* Right cluster */}
          <div className="flex items-center gap-2 ml-auto">
            {accessToken && (
              <CreateEventDialog>
                <Plus className="h-4 w-4" />
                <span className="hidden sm:inline">Создать событие</span>
              </CreateEventDialog>
            )}

            {accessToken ? (
              <>
                <NotificationBell />

                <DropdownMenu>
                  <DropdownMenuTrigger className="relative h-10 w-10 rounded-full inline-flex items-center justify-center hover:bg-accent/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                    <Avatar className="h-9 w-9">
                      <AvatarImage src={avatarUrl(me?.avatar_url) ?? ""} alt={me?.username ?? "user"} />
                      <AvatarFallback className="bg-primary text-primary-foreground">
                        {me?.username?.[0]?.toUpperCase() ?? "?"}
                      </AvatarFallback>
                    </Avatar>
                  </DropdownMenuTrigger>

                  <DropdownMenuContent align="end" className="w-56">
                    <DropdownMenuItem
                      onClick={() => {
                        const me = accessToken
                          ? decodeJwt(accessToken)?.user_id
                          : null;
                        if (me) router.push(`/users/${me}`);
                        else router.push("/login");
                      }}
                    >
                      Профиль
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => router.push("/settings")}>
                      Настройки
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={() => void logout()}>
                      Выйти
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </>
            ) : (
              <div className="flex items-center gap-2">
                <Link
                  href="/login"
                  className={buttonVariants({ variant: "ghost" })}
                >
                  Войти
                </Link>
                <Link
                  href="/register"
                  className={buttonVariants({ variant: "outline" })}
                >
                  Регистрация
                </Link>
              </div>
            )}
          </div>
        </div>
      </header>

      {/* Мобильное меню (Sheet с сайдбаром) */}
      <Sheet open={mobileMenuOpen} onOpenChange={setMobileMenuOpen}>
        <SheetContent side="left" className="p-0 w-[85vw] max-w-[320px] sm:w-[320px]">
          <SheetTitle className="sr-only">Меню навигации</SheetTitle>
          <AppSidebar isMobile onNavigate={() => setMobileMenuOpen(false)} />
        </SheetContent>
      </Sheet>
    </>
  );
}