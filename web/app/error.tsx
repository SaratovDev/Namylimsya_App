"use client";

import { useEffect } from "react";
import Link from "next/link";
import { AlertTriangle, RefreshCw } from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";

// Root error boundary — ловит ошибки в любом сегменте.
// Рендерится ВМЕСТО сломанного компонента, layout остаётся (но без sidebar,
// потому что это вне (app) — обёртка не сработает).
// Специально НЕ используем UI-kit с sidebar — тут может не быть контекста.
export default function RootError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Логируем в консоль для отладки (в проде — отправить в Sentry).
    console.error("App error:", error);
  }, [error]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="max-w-md w-full text-center space-y-4">
        <div className="flex justify-center">
          <AlertTriangle className="h-16 w-16 text-destructive" />
        </div>
        <h1 className="text-3xl font-bold">Что-то сломалось</h1>
        <p className="text-muted-foreground">
          Произошла ошибка. Мы уже знаем о ней. Попробуйте обновить страницу или
          вернуться на главную.
        </p>
        {process.env.NODE_ENV === "development" && error.message && (
          <div className="text-left p-3 rounded-lg bg-destructive/5 border border-destructive/20">
            <p className="text-xs font-mono text-destructive break-words">
              {error.message}
            </p>
          </div>
        )}
        <div className="flex flex-col sm:flex-row gap-2 justify-center">
          <Button onClick={() => reset()} className="gap-2">
            <RefreshCw className="h-4 w-4" />
            Попробовать снова
          </Button>
          <Link href="/" className={buttonVariants({ variant: "outline" })}>
            На главную
          </Link>
        </div>
      </div>
    </div>
  );
}