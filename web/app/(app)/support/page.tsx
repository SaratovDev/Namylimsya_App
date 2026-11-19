"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Loader2, Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

const TICKET_TYPES = [
  { value: "suggestion", label: "Предложение", description: "Идея, как сделать лучше" },
  { value: "complaint", label: "Жалоба", description: "Нарушение правил, спам, оскорбления" },
  { value: "appeal", label: "Апелляция", description: "Оспорить бан или блокировку" },
] as const;

type TicketType = (typeof TICKET_TYPES)[number]["value"];

export default function SupportPage() {
  const router = useRouter();
  const { accessToken, isReady } = useAuth();
  const [type, setType] = useState<TicketType>("suggestion");
  const [subject, setSubject] = useState("");
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(false);

  const subjectOk = subject.trim().length > 0;
  const messageOk = message.trim().length >= 10;
  const canSubmit = subjectOk && messageOk && !loading;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    setLoading(true);
    try {
      await apiFetch(
        "/support/tickets",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            type,
            subject: subject.trim(),
            message: message.trim(),
          }),
        },
        accessToken
      );
      toast.success("Обращение отправлено. Мы свяжемся с вами.");
      setSubject("");
      setMessage("");
      router.push("/");
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Не удалось отправить";
      if (msg.includes("rate_limit") || msg.includes("too many")) {
        toast.error("Слишком много обращений. Попробуйте позже.");
      } else {
        toast.error(msg);
      }
    } finally {
      setLoading(false);
    }
  };

  if (!isReady) {
    return (
      <div className="max-w-2xl mx-auto py-6 px-4 flex items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="max-w-2xl mx-auto py-6 px-4">
      <h1 className="text-3xl font-bold mb-2">Поддержка</h1>
      <p className="text-sm text-muted-foreground mb-6">
        Опишите проблему или идею — мы прочитаем и ответим на указанный при
        регистрации email.
      </p>

      {!accessToken && (
        <Card className="p-4 mb-6 bg-muted/30">
          <p className="text-sm text-muted-foreground">
            Обращение можно отправить и без входа — но{" "}
            <Link href="/login" className="text-primary hover:underline">
              войдите
            </Link>
            , чтобы мы могли связать его с вашим аккаунтом и ответить быстрее.
          </p>
        </Card>
      )}

      <form onSubmit={handleSubmit} className="space-y-6">
        <div className="space-y-2">
          <Label>Тип обращения</Label>
          <div className="space-y-2">
            {TICKET_TYPES.map((t) => (
              <label
                key={t.value}
                className="flex items-start gap-3 p-3 rounded-lg border cursor-pointer hover:bg-muted/30 transition-colors"
              >
                <input
                  type="radio"
                  name="ticket-type"
                  value={t.value}
                  checked={type === t.value}
                  onChange={() => setType(t.value)}
                  disabled={loading}
                  className="mt-1 accent-primary"
                />
                <div className="flex-1">
                  <div className="text-sm font-medium">{t.label}</div>
                  <div className="text-xs text-muted-foreground">{t.description}</div>
                </div>
              </label>
            ))}
          </div>
        </div>

        <div className="space-y-2">
          <Label htmlFor="subject">
            Тема <span className="text-destructive">*</span>
          </Label>
          <Input
            id="subject"
            value={subject}
            onChange={(e) => setSubject(e.target.value)}
            placeholder="Коротко о проблеме"
            maxLength={300}
            disabled={loading}
            required
          />
          <p className="text-xs text-muted-foreground text-right">{subject.length}/300</p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="message">
            Сообщение <span className="text-destructive">*</span>
          </Label>
          <Textarea
            id="message"
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            placeholder="Опишите ситуацию подробно, приложите ссылки на события или профили, если есть."
            rows={6}
            maxLength={5000}
            disabled={loading}
            required
          />
          <p className="text-xs text-muted-foreground text-right">
            {message.length}/5000 · минимум 10 символов
          </p>
        </div>

        <Button type="submit" disabled={!canSubmit} className="w-full gap-2">
          {loading ? (
            <>
              <Loader2 className="h-4 w-4 animate-spin" />
              Отправка…
            </>
          ) : (
            <>
              <Send className="h-4 w-4" />
              Отправить обращение
            </>
          )}
        </Button>
      </form>
    </div>
  );
}