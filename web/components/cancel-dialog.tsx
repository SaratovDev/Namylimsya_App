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
import {
  CancelReason,
  EVENT_CANCEL_REASONS,
  APPLICATION_CANCEL_REASONS,
  resolveCancelReason,
} from "@/lib/cancel-reasons";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  variant: "event" | "application";
  onConfirm: (reason: string) => Promise<void> | void;
  loading?: boolean;
};

export function CancelDialog({
  open,
  onOpenChange,
  variant,
  onConfirm,
  loading = false,
}: Props) {
  const reasons: CancelReason[] =
    variant === "event" ? EVENT_CANCEL_REASONS : APPLICATION_CANCEL_REASONS;

  const [selected, setSelected] = useState<string>(reasons[0].value);
  const [customText, setCustomText] = useState("");
  const [localLoading, setLocalLoading] = useState(false);

  const isOther = selected === "other";
  const canConfirm =
    !localLoading && !loading && (!isOther || customText.trim().length > 0);

  const handleConfirm = async () => {
    if (!canConfirm) return;
    const reason = resolveCancelReason(selected, customText, reasons);
    setLocalLoading(true);
    try {
      await onConfirm(reason);
      // Сброс
      setSelected(reasons[0].value);
      setCustomText("");
    } finally {
      setLocalLoading(false);
    }
  };

  const handleClose = (next: boolean) => {
    if (!next) {
      setSelected(reasons[0].value);
      setCustomText("");
    }
    onOpenChange(next);
  };

  const busy = localLoading || loading;

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>
            {variant === "event" ? "Отменить событие?" : "Отменить заявку?"}
          </DialogTitle>
          <DialogDescription>
            {variant === "event"
              ? "Все участники получат уведомление. Это действие нельзя отменить."
              : "Заявка будет переведена в статус «Отменено». Подать её снова можно позже."}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 mt-2">
          <Label>Причина</Label>
          <div className="space-y-2">
            {reasons.map((r) => (
              <label
                key={r.value}
                className="flex items-center gap-2 text-sm cursor-pointer"
              >
                <input
                  type="radio"
                  name="cancel-reason"
                  value={r.value}
                  checked={selected === r.value}
                  onChange={() => setSelected(r.value)}
                  className="h-4 w-4 accent-primary"
                  disabled={busy}
                />
                <span>{r.label}</span>
              </label>
            ))}
          </div>

          {isOther && (
            <div className="pt-2">
              <Input
                placeholder={
                  variant === "event"
                    ? "Опишите причину отмены"
                    : "Опишите причину отмены заявки"
                }
                value={customText}
                onChange={(e) => setCustomText(e.target.value)}
                disabled={busy}
                maxLength={200}
                autoFocus
              />
              <p className="text-xs text-muted-foreground mt-1">
                {customText.trim().length}/200
              </p>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button
            variant="ghost"
            onClick={() => handleClose(false)}
            disabled={busy}
          >
            Отмена
          </Button>
          <Button
            variant="destructive"
            onClick={handleConfirm}
            disabled={!canConfirm}
          >
            {busy ? "Отправка..." : "Подтвердить"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}