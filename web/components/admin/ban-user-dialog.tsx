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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

type Duration = "1h" | "24h" | "7d" | "30d" | "forever" | "custom";

const DURATION_LABELS: Record<Duration, string> = {
  "1h": "1 час",
  "24h": "24 часа",
  "7d": "7 дней",
  "30d": "30 дней",
  forever: "Навсегда",
  custom: "Своя дата",
};

function computeExpires(d: Duration, custom: string): string | undefined {
  if (d === "forever") return undefined;
  if (d === "custom") {
    if (!custom) return undefined;
    // Из "YYYY-MM-DDTHH:mm" → RFC3339
    return new Date(custom).toISOString();
  }
  const now = new Date();
  switch (d) {
    case "1h":  now.setHours(now.getHours() + 1); break;
    case "24h": now.setDate(now.getDate() + 1); break;
    case "7d":  now.setDate(now.getDate() + 7); break;
    case "30d": now.setDate(now.getDate() + 30); break;
  }
  return now.toISOString();
}

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  username: string;
  onConfirm: (reason: string, expiresAt?: string) => Promise<void> | void;
  loading?: boolean;
};

export function BanUserDialog({ open, onOpenChange, username, onConfirm, loading }: Props) {
  const [reason, setReason] = useState("");
  const [duration, setDuration] = useState<Duration>("7d");
  const [customDate, setCustomDate] = useState("");
  const [localLoading, setLocalLoading] = useState(false);

  // Сброс формы при закрытии диалога — корректировка состояния при рендере
  // (React-паттерн вместо setState в useEffect).
  const [prevOpen, setPrevOpen] = useState(open);
  if (prevOpen !== open) {
    setPrevOpen(open);
    if (!open) {
      setReason("");
      setDuration("7d");
      setCustomDate("");
    }
  }

  const canConfirm =
    reason.trim().length > 0 &&
    !localLoading &&
    !loading &&
    (duration !== "custom" || customDate);

  const handleConfirm = async () => {
    if (!canConfirm) return;
    setLocalLoading(true);
    try {
      await onConfirm(reason.trim(), computeExpires(duration, customDate));
    } finally {
      setLocalLoading(false);
    }
  };

  const busy = localLoading || loading;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Забанить @{username}?</DialogTitle>
          <DialogDescription>
            Пользователь потеряет доступ и все активные сессии. Действие логируется.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="ban-reason">Причина бана <span className="text-destructive">*</span></Label>
            <Textarea
              id="ban-reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="Например: спам в чатах"
              maxLength={500}
              rows={3}
              disabled={busy}
            />
            <p className="text-xs text-muted-foreground text-right">{reason.length}/500</p>
          </div>

          <div className="space-y-2">
            <Label>Срок</Label>
            <div className="grid grid-cols-3 gap-1.5">
              {(Object.keys(DURATION_LABELS) as Duration[]).map((d) => (
                <Button
                  key={d}
                  type="button"
                  variant={duration === d ? "default" : "outline"}
                  size="sm"
                  onClick={() => setDuration(d)}
                  disabled={busy}
                  className="text-xs"
                >
                  {DURATION_LABELS[d]}
                </Button>
              ))}
            </div>
            {duration === "custom" && (
              <Input
                type="datetime-local"
                value={customDate}
                onChange={(e) => setCustomDate(e.target.value)}
                disabled={busy}
                min={new Date().toISOString().slice(0, 16)}
              />
            )}
          </div>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>
            Отмена
          </Button>
          <Button variant="destructive" onClick={handleConfirm} disabled={!canConfirm}>
            {busy ? "Бан..." : "Забанить"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}