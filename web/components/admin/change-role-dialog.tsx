"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";

type Role = "user" | "moderator" | "admin";

const ROLE_LABELS: Record<Role, string> = {
  user: "Пользователь",
  moderator: "Модератор",
  admin: "Администратор",
};

const ROLE_HINTS: Record<Role, string> = {
  user: "Обычный доступ",
  moderator: "Бан/разбан, жалобы, тикеты, отмена событий",
  admin: "Полный доступ, включая смену ролей и удаление",
};

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  username: string;
  currentRole: Role;
  onConfirm: (role: Role) => Promise<void> | void;
  loading?: boolean;
};

export function ChangeRoleDialog({
  open,
  onOpenChange,
  username,
  currentRole,
  onConfirm,
  loading,
}: Props) {
  const [role, setRole] = useState<Role>(currentRole);
  const [localLoading, setLocalLoading] = useState(false);

  // Синхронизация роли с текущей при открытии — корректировка при рендере.
  const [prevOpen, setPrevOpen] = useState(open);
  if (prevOpen !== open) {
    setPrevOpen(open);
    if (open) setRole(currentRole);
  }

  const handleConfirm = async () => {
    if (role === currentRole) {
      onOpenChange(false);
      return;
    }
    setLocalLoading(true);
    try {
      await onConfirm(role);
    } finally {
      setLocalLoading(false);
    }
  };

  const busy = localLoading || loading;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Изменить роль @{username}</DialogTitle>
          <DialogDescription>
            Роль определяет доступ к админ-функциям. Действие логируется.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-2">
          <Label>Новая роль</Label>
          {(Object.keys(ROLE_LABELS) as Role[]).map((r) => (
            <label
              key={r}
              className="flex items-start gap-3 p-2.5 rounded-md border cursor-pointer hover:bg-muted/50 transition-colors"
            >
              <input
                type="radio"
                name="role"
                value={r}
                checked={role === r}
                onChange={() => setRole(r)}
                disabled={busy}
                className="mt-1 accent-primary"
              />
              <div className="flex-1">
                <div className="text-sm font-medium">{ROLE_LABELS[r]}</div>
                <div className="text-xs text-muted-foreground">{ROLE_HINTS[r]}</div>
              </div>
              {r === currentRole && (
                <span className="text-xs text-muted-foreground shrink-0">текущая</span>
              )}
            </label>
          ))}
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>
            Отмена
          </Button>
          <Button onClick={handleConfirm} disabled={busy || role === currentRole}>
            {busy ? "..." : "Применить"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}