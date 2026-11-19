"use client";

import { useEffect } from "react";
import Link from "next/link";
import { AlertTriangle, RefreshCw } from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";

// Error boundary ВНУТРИ (app) — сайдбар и шапка остаются, ломается только контент.
// Рендерится вместо children, но layout (app) сохранён.
export default function AppError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("App error:", error);
  }, [error]);

  return (
    <div className="max-w-2xl mx-auto py-16 px-4">
      <div className="text-center space-y-4">
        <div className="flex justify-center">
          <AlertTriangle className="h-16 w-16 text-destructive" />
        </div>
        <h1 className="text-2xl font-bold">Не удалось загрузить</h1>
        <p className="text-muted-foreground">
          Что-то пошло не так. Попробуйте обновить страницу.
        </p>
        {process.env.NODE_ENV === "development" && error.message && (
          <div className="text-left p-3 rounded-lg bg-destructive/5 border border-destructive/20">
            <p className="text-xs font-mono text-destructive break-words">
              {error.message}
            </p>
          </div>
        )}
        <div className="flex flex-col sm:flex-row gap-2 justify-center pt-4">
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