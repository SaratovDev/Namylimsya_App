"use client";

import { useState } from "react";
import { toast } from "sonner";
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
import { Textarea } from "@/components/ui/textarea";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  userId: string;
  username: string;
};

const MIN_REASON = 10;
const MAX_REASON = 500;

export function ReportUserDialog({
  open,
  onOpenChange,
  userId,
  username,
}: Props) {
  const { accessToken } = useAuth();
  const [reason, setReason] = useState("");
  const [loading, setLoading] = useState(false);

  const [prevOpen, setPrevOpen] = useState(open);
  if (prevOpen !== open) {
    setPrevOpen(open);
    if (!open) setReason("");
  }

  const trimmed = reason.trim();
  const canSubmit =
    trimmed.length >= MIN_REASON && trimmed.length <= MAX_REASON && !loading;

  const handleSubmit = async () => {
    if (!canSubmit) return;
    if (!accessToken) {
      toast.error("Войдите, чтобы пожаловаться");
      return;
    }
    setLoading(true);
    try {
      await apiFetch(
        "/complaints",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            target_type: "user",
            target_id: userId,
            reason: trimmed,
          }),
        },
        accessToken
      );
      toast.success("Жалоба отправлена. Мы рассмотрим её в ближайшее время.");
      onOpenChange(false);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Не удалось отправить жалобу"
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Пожаловаться на @{username}</DialogTitle>
          <DialogDescription>
            Опишите, что не так. Жалоба уйдёт модераторам — они рассмотрят её и
            примут меры, если нарушения подтвердятся.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-2">
          <Label htmlFor="report-reason">Причина</Label>
          <Textarea
            id="report-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Например: оскорбления в чате события, спам в профиле…"
            maxLength={MAX_REASON}
            rows={4}
            disabled={loading}
            autoFocus
          />
          <p className="text-xs text-muted-foreground text-right">
            {reason.length}/{MAX_REASON}
          </p>
        </div>

        <DialogFooter>
          <Button
            variant="ghost"
            onClick={() => onOpenChange(false)}
            disabled={loading}
          >
            Отмена
          </Button>
          <Button
            variant="destructive"
            onClick={handleSubmit}
            disabled={!canSubmit}
          >
            {loading ? "Отправка…" : "Отправить жалобу"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}