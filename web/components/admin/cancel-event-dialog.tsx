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

const REASONS = [
  { value: "violation", label: "Нарушение правил" },
  { value: "spam", label: "Спам или мошенничество" },
  { value: "duplicate", label: "Дубликат" },
  { value: "report", label: "По жалобе пользователей" },
  { value: "other", label: "Своя причина" },
] as const;

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  eventTitle: string;
  onConfirm: (reason: string) => Promise<void> | void;
  loading?: boolean;
};

export function CancelEventDialog({
  open,
  onOpenChange,
  eventTitle,
  onConfirm,
  loading,
}: Props) {
  const [selected, setSelected] = useState<string>(REASONS[0].value);
  const [custom, setCustom] = useState("");
  const [localLoading, setLocalLoading] = useState(false);

  // Сброс причины при закрытии — корректировка состояния при рендере.
  const [prevOpen, setPrevOpen] = useState(open);
  if (prevOpen !== open) {
    setPrevOpen(open);
    if (!open) {
      setSelected(REASONS[0].value);
      setCustom("");
    }
  }

  const isOther = selected === "other";
  const resolvedReason = isOther ? custom.trim() : REASONS.find((r) => r.value === selected)?.label ?? "";
  const canConfirm = resolvedReason.length > 0 && !localLoading && !loading;

  const handleConfirm = async () => {
    if (!canConfirm) return;
    setLocalLoading(true);
    try {
      await onConfirm(resolvedReason);
    } finally {
      setLocalLoading(false);
    }
  };

  const busy = localLoading || loading;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Отменить событие?</DialogTitle>
          <DialogDescription>
            «{eventTitle}» будет отменено, все участники получат уведомление.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-2">
          <Label>Причина отмены</Label>
          {REASONS.map((r) => (
            <label
              key={r.value}
              className="flex items-center gap-2 text-sm cursor-pointer py-1"
            >
              <input
                type="radio"
                name="admin-cancel-reason"
                value={r.value}
                checked={selected === r.value}
                onChange={() => setSelected(r.value)}
                disabled={busy}
                className="accent-primary"
              />
              {r.label}
            </label>
          ))}
          {isOther && (
            <Input
              placeholder="Опишите причину"
              value={custom}
              onChange={(e) => setCustom(e.target.value)}
              disabled={busy}
              maxLength={300}
              autoFocus
            />
          )}
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>
            Отмена
          </Button>
          <Button variant="destructive" onClick={handleConfirm} disabled={!canConfirm}>
            {busy ? "Отмена..." : "Отменить событие"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}