"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { CheckCircle2, XCircle, Loader2 } from "lucide-react";
import { apiFetch } from "@/lib/api";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { buttonVariants } from "@/components/ui/button";

function VerifyEmailInner() {
  const params = useSearchParams();
  const token = params.get("token");
  const [status, setStatus] = useState<"loading" | "ok" | "error">(
    token ? "loading" : "error"
  );

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch(`/auth/verify-email?token=${encodeURIComponent(token)}`, {}, null)
      .then(() => {
        if (!cancelled) setStatus("ok");
      })
      .catch(() => {
        if (!cancelled) setStatus("error");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  return (
    <Card className="w-full">
      <CardHeader className="text-center">
        <CardTitle className="text-2xl">Подтверждение email</CardTitle>
      </CardHeader>
      <CardContent className="text-center space-y-4">
        {status === "loading" && (
          <div className="flex items-center justify-center gap-2 text-muted-foreground">
            <Loader2 className="h-5 w-5 animate-spin" />
            Проверяем ссылку…
          </div>
        )}
        {status === "ok" && (
          <>
            <CheckCircle2 className="h-10 w-10 text-green-500 mx-auto" />
            <p className="text-sm">Email подтверждён. Спасибо!</p>
            <Link href="/" className={buttonVariants()}>
              Перейти к событиям
            </Link>
          </>
        )}
        {status === "error" && (
          <>
            <XCircle className="h-10 w-10 text-destructive mx-auto" />
            <CardDescription>
              Ссылка недействительна или истекла. Запросите новую в настройках
              профиля.
            </CardDescription>
            <Link href="/login" className={buttonVariants()}>
              Войти
            </Link>
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default function VerifyEmailPage() {
  return (
    <Suspense
      fallback={
        <div className="flex justify-center py-10">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      }
    >
      <VerifyEmailInner />
    </Suspense>
  );
}
