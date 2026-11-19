"use client";

import { useEffect } from "react";
import Link from "next/link";
import { AlertTriangle } from "lucide-react";
import { isLegalEntityConfigured } from "@/lib/legal";

// LegalPlaceholderBanner показывает заметное предупреждение, пока в
// LEGAL_ENTITY остаются плейсхолдеры вида [ФАМИЛИЯ], [ИНН] и т.п.
// Это защита от случайного деплоя на прод: 152-ФЗ требует публиковать
// реквизиты оператора, а не шаблон.
//
// Компонент клиентский: использует useEffect для console.warn в dev
// (чтобы разработчик увидел проблему и в консоли, а не только глазами).
// При заполненных реквизитах возвращает null — ничего не рендерит.
export function LegalPlaceholderBanner() {
  const configured = isLegalEntityConfigured();

  useEffect(() => {
    if (!configured && process.env.NODE_ENV === "development") {
      console.warn(
        "[legal] LEGAL_ENTITY содержит плейсхолдеры ([...]). " +
          "Заполните web/lib/legal.ts перед публичным запуском."
      );
    }
  }, [configured]);

  if (configured) {
    return null;
  }

  return (
    <div
      role="alert"
      className="mb-6 rounded-xl border-2 border-destructive/60 bg-destructive/10 p-4 text-sm"
    >
      <div className="flex items-start gap-3">
        <AlertTriangle className="h-5 w-5 shrink-0 text-destructive mt-0.5" />
        <div className="flex-1 min-w-0">
          <p className="font-semibold text-destructive">
            Документ не готов к публикации
          </p>
          <p className="mt-1 text-foreground/80">
            В реквизитах оператора остались плейсхолдеры (
            <code className="px-1 rounded bg-muted font-mono text-xs">
              [ФАМИЛИЯ]
            </code>
            ,{" "}
            <code className="px-1 rounded bg-muted font-mono text-xs">
              [ИНН]
            </code>{" "}
            и т.п.). Заполните{" "}
            <code className="px-1 rounded bg-muted font-mono text-xs">
              web/lib/legal.ts
            </code>{" "}
            перед деплоем — иначе это нарушение 152-ФЗ.
          </p>
          <p className="mt-2 text-xs text-muted-foreground">
            Баннер исчезнет автоматически, когда все поля{" "}
            <code className="px-1 rounded bg-muted font-mono">LEGAL_ENTITY</code>{" "}
            будут без квадратных скобок.
          </p>
        </div>
        <Link
          href="/"
          className="text-xs text-muted-foreground hover:text-foreground shrink-0"
        >
          На главную
        </Link>
      </div>
    </div>
  );
}