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

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  userEmail: string | null;
  subject: string;
  onConfirm: (body: string) => Promise<void> | void;
  loading?: boolean;
};

export function ReplyTicketDialog({
  open,
  onOpenChange,
  userEmail,
  subject,
  onConfirm,
  loading,
}: Props) {
  const [body, setBody] = useState("");
  const [localLoading, setLocalLoading] = useState(false);

  const [prevOpen, setPrevOpen] = useState(open);
  if (prevOpen !== open) {
    setPrevOpen(open);
    if (!open) setBody("");
  }

  const trimmed = body.trim();
  const canConfirm = trimmed.length > 0 && !localLoading && !loading;

  const handleConfirm = async () => {
    if (!canConfirm) return;
    setLocalLoading(true);
    try {
      await onConfirm(trimmed);
    } finally {
      setLocalLoading(false);
    }
  };

  const busy = localLoading || loading;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Ответ на тикет</DialogTitle>
          <DialogDescription>
            {userEmail
              ? `Ответ будет отправлен на ${userEmail} и сохранён в истории тикета.`
              : "У тикета нет email — ответ будет сохранён в истории, письмо не отправится."}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-2">
          <Label>Тема: «{subject}»</Label>
          <Label htmlFor="ticket-reply">Текст ответа</Label>
          <Textarea
            id="ticket-reply"
            value={body}
            onChange={(e) => setBody(e.target.value)}
            placeholder="Введите ответ пользователю"
            maxLength={5000}
            rows={6}
            disabled={busy}
            autoFocus
          />
          <p className="text-xs text-muted-foreground text-right">
            {body.length}/5000
          </p>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>
            Отмена
          </Button>
          <Button onClick={handleConfirm} disabled={!canConfirm}>
            {busy ? "Отправка..." : "Отправить ответ"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}