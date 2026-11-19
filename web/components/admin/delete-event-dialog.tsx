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

const CONFIRM_WORD = "УДАЛИТЬ";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  eventTitle: string;
  onConfirm: () => Promise<void> | void;
  loading?: boolean;
};

export function DeleteEventDialog({
  open,
  onOpenChange,
  eventTitle,
  onConfirm,
  loading,
}: Props) {
  const [input, setInput] = useState("");
  const [localLoading, setLocalLoading] = useState(false);

  const [prevOpen, setPrevOpen] = useState(open);
  if (prevOpen !== open) {
    setPrevOpen(open);
    if (!open) setInput("");
  }

  const canConfirm = input.trim() === CONFIRM_WORD && !localLoading && !loading;

  const handleConfirm = async () => {
    if (!canConfirm) return;
    setLocalLoading(true);
    try {
      await onConfirm();
    } finally {
      setLocalLoading(false);
    }
  };

  const busy = localLoading || loading;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="text-destructive">
            Удалить событие навсегда?
          </DialogTitle>
          <DialogDescription>
            «{eventTitle}» и все связанные данные (заявки, чат) будут удалены
            безвозвратно. Действие необратимо.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-2">
          <Label htmlFor="confirm-delete">
            Введите <span className="font-mono font-bold">{CONFIRM_WORD}</span> для подтверждения
          </Label>
          <Input
            id="confirm-delete"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={CONFIRM_WORD}
            disabled={busy}
            autoComplete="off"
          />
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>
            Отмена
          </Button>
          <Button variant="destructive" onClick={handleConfirm} disabled={!canConfirm}>
            {busy ? "Удаление..." : "Удалить навсегда"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}