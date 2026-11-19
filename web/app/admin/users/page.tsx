"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  Search,
  AlertCircle,
  Loader2,
  ChevronLeft,
  ChevronRight,
  ShieldAlert,
  Ban,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { AdminGuard } from "@/components/admin-guard";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { AdminUserListItem } from "@/lib/types";

const PAGE_SIZE = 25;

export default function Page() {
  return (
    <AdminGuard allow={["admin", "moderator"]}>
      <UsersList />
    </AdminGuard>
  );
}

function UsersList() {
  const { accessToken, isReady } = useAuth();
  const [users, setUsers] = useState<AdminUserListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [roleFilter, setRoleFilter] = useState<"" | "user" | "moderator" | "admin">("");
  const [offset, setOffset] = useState(0);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    if (!isReady || !accessToken) return;
    let cancelled = false;

    (async () => {
      setLoading(true);
      setError(null);
      try {
        const params = new URLSearchParams();
        if (search.trim()) params.set("search", search.trim());
        if (roleFilter) params.set("role", roleFilter);
        params.set("limit", String(PAGE_SIZE));
        params.set("offset", String(offset));

        const res = await apiFetch(`/admin/users?${params.toString()}`, {}, accessToken);
        const json = (await res.json()) as { users: AdminUserListItem[] };
        if (!cancelled) setUsers(json.users ?? []);
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Не удалось загрузить");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [isReady, accessToken, search, roleFilter, offset, reloadKey]);

  const reload = useCallback(() => setReloadKey((k) => k + 1), []);

  const applySearch = () => {
    setOffset(0);
    setReloadKey((k) => k + 1);
  };

  const hasPrev = offset > 0;
  const hasNext = users.length === PAGE_SIZE;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Пользователи</h1>
        <Button variant="ghost" size="sm" onClick={reload} disabled={loading} className="gap-2">
          {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
          Обновить
        </Button>
      </div>

      {/* Фильтры */}
      <div className="flex flex-col sm:flex-row gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
          <Input
            placeholder="Поиск по email или username"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && applySearch()}
            className="pl-9"
          />
        </div>
        <select
          value={roleFilter}
          onChange={(e) => {
            setRoleFilter(e.target.value as typeof roleFilter);
            setOffset(0);
          }}
          className="h-9 rounded-md border border-input bg-background px-3 text-sm"
        >
          <option value="">Все роли</option>
          <option value="user">user</option>
          <option value="moderator">moderator</option>
          <option value="admin">admin</option>
        </select>
        <Button onClick={applySearch}>Найти</Button>
      </div>

      {/* Список */}
      {loading && users.length === 0 ? (
        <div className="space-y-2">
          {[1, 2, 3, 4, 5].map((i) => (
            <Skeleton key={i} className="h-14 w-full rounded-lg" />
          ))}
        </div>
      ) : error ? (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <AlertCircle className="h-10 w-10 text-destructive mb-3" />
          <p className="text-sm text-muted-foreground mb-4">{error}</p>
          <Button variant="outline" onClick={reload}>Повторить</Button>
        </div>
      ) : users.length === 0 ? (
        <div className="py-12 text-center text-sm text-muted-foreground">
          Пользователи не найдены
        </div>
      ) : (
        <div className="border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-muted/50 border-b">
              <tr>
                <th className="text-left px-4 py-2 font-medium">Пользователь</th>
                <th className="text-left px-4 py-2 font-medium hidden md:table-cell">Email</th>
                <th className="text-left px-4 py-2 font-medium">Роль</th>
                <th className="text-left px-4 py-2 font-medium">Статус</th>
                <th className="text-left px-4 py-2 font-medium hidden lg:table-cell">Регистрация</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id} className="border-b last:border-0 hover:bg-muted/30 transition-colors">
                  <td className="px-4 py-2">
                    <Link
                      href={`/admin/users/${u.id}`}
                      className="font-medium hover:text-primary transition-colors"
                    >
                      @{u.username}
                    </Link>
                    <div className="text-xs text-muted-foreground md:hidden">{u.email}</div>
                  </td>
                  <td className="px-4 py-2 text-muted-foreground hidden md:table-cell">{u.email}</td>
                  <td className="px-4 py-2">
                    <Badge variant={u.role === "admin" ? "default" : u.role === "moderator" ? "secondary" : "outline"} className="text-xs">
                      {u.role === "admin" && <ShieldAlert className="h-3 w-3 mr-1" />}
                      {u.role}
                    </Badge>
                  </td>
                  <td className="px-4 py-2">
                    {u.is_banned ? (
                      <Badge variant="destructive" className="text-xs">
                        <Ban className="h-3 w-3 mr-1" />
                        забанен
                      </Badge>
                    ) : (
                      <span className="text-xs text-muted-foreground">активен</span>
                    )}
                  </td>
                  <td className="px-4 py-2 text-xs text-muted-foreground hidden lg:table-cell">
                    {new Date(u.created_at).toLocaleDateString("ru-RU")}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Pagination */}
      <div className="flex items-center justify-between">
        <div className="text-xs text-muted-foreground">
          {users.length > 0 && `Показано ${offset + 1}–${offset + users.length}`}
        </div>
        <div className="flex gap-1">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
            disabled={!hasPrev || loading}
            className="gap-1"
          >
            <ChevronLeft className="h-4 w-4" />
            Назад
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setOffset(offset + PAGE_SIZE)}
            disabled={!hasNext || loading}
            className="gap-1"
          >
            Вперёд
            <ChevronRight className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}