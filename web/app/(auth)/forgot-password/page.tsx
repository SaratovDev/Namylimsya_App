"use client";

import { useState } from "react";
import Link from "next/link";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { apiFetch } from "@/lib/api";

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [loading, setLoading] = useState(false);
  const [sent, setSent] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await apiFetch(
        "/auth/password-reset-request",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ email }),
        },
        null
      );
      setSent(true);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Не удалось отправить запрос"
      );
    } finally {
      setLoading(false);
    }
  };

  if (sent) {
    return (
      <Card className="w-full">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">Проверьте почту</CardTitle>
          <CardDescription>
            Если <strong>{email}</strong> зарегистрирован, на него отправлен
            код для сброса пароля.
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          Код действует 15 минут. Не пришло? Проверьте «Спам» или{" "}
          <button
            onClick={() => setSent(false)}
            className="text-primary hover:underline"
          >
            отправьте ещё раз
          </button>
          .
        </CardContent>
        <CardFooter className="flex flex-col gap-2">
          <Link
            href={`/reset-password?email=${encodeURIComponent(email)}`}
            className="text-primary hover:underline text-sm"
          >
            У меня есть код →
          </Link>
          <Link
            href="/login"
            className="text-sm text-muted-foreground hover:text-foreground"
          >
            Вернуться ко входу
          </Link>
        </CardFooter>
      </Card>
    );
  }

  return (
    <Card className="w-full">
      <CardHeader className="text-center">
        <CardTitle className="text-2xl">Забыли пароль?</CardTitle>
        <CardDescription>
          Введите email — мы отправим код для сброса
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="email">Email</Label>
            <Input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com"
              required
            />
          </div>
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Отправка…
              </>
            ) : (
              "Отправить код"
            )}
          </Button>
        </form>
      </CardContent>
      <CardFooter className="justify-center text-sm text-muted-foreground">
        <Link href="/login" className="text-primary hover:underline">
          Вернуться ко входу
        </Link>
      </CardFooter>
    </Card>
  );
}