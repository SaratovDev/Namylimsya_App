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
import { Textarea } from "@/components/ui/textarea";

type Mode = "resolve" | "reject";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mode: Mode;
  onConfirm: (comment: string) => Promise<void> | void;
  loading?: boolean;
};

export function TicketActionDialog({
  open,
  onOpenChange,
  mode,
  onConfirm,
  loading,
}: Props) {
  const [comment, setComment] = useState("");
  const [localLoading, setLocalLoading] = useState(false);

  const [prevOpen, setPrevOpen] = useState(open);
  if (prevOpen !== open) {
    setPrevOpen(open);
    if (!open) setComment("");
  }

  const handleConfirm = async () => {
    setLocalLoading(true);
    try {
      await onConfirm(comment.trim());
    } finally {
      setLocalLoading(false);
    }
  };

  const busy = localLoading || loading;
  const isResolve = mode === "resolve";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>
            {isResolve ? "Закрыть тикет как решённый?" : "Отклонить тикет?"}
          </DialogTitle>
          <DialogDescription>
            {isResolve
              ? "Тикет будет закрыт. При необходимости сначала отправьте ответ через «Ответить»."
              : "Тикет будет отклонён без ответа пользователю."}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-2">
          <Label htmlFor="ticket-comment">Комментарий (внутренний)</Label>
          <Textarea
            id="ticket-comment"
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            placeholder={
              isResolve
                ? "Например: вопрос решён, ответ отправлен"
                : "Например: не требует внимания"
            }
            maxLength={1000}
            rows={3}
            disabled={busy}
          />
          <p className="text-xs text-muted-foreground text-right">
            {comment.length}/1000
          </p>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>
            Отмена
          </Button>
          <Button
            variant={isResolve ? "default" : "destructive"}
            onClick={handleConfirm}
            disabled={busy}
          >
            {busy ? "..." : isResolve ? "Закрыть" : "Отклонить"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}