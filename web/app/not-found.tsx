import Link from "next/link";
import { SearchX } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";

// Глобальная 404 — рендерится для любого notFound() или несуществующего URL.
// Без "use client" — статическая страница, рендерится на сервере.
export default function NotFound() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="max-w-md w-full text-center space-y-4">
        <div className="flex justify-center">
          <SearchX className="h-16 w-16 text-muted-foreground" />
        </div>
        <h1 className="text-6xl font-bold">404</h1>
        <h2 className="text-2xl font-semibold">Страница не найдена</h2>
        <p className="text-muted-foreground">
          Возможно, она была удалена, переименована, или вы ошиблись адресом.
        </p>
        <div className="flex flex-col sm:flex-row gap-2 justify-center pt-4">
          <Link href="/" className={buttonVariants()}>
            На главную
          </Link>
          <Link
            href="/events"
            className={buttonVariants({ variant: "outline" })}
          >
            К событиям
          </Link>
        </div>
      </div>
    </div>
  );
}