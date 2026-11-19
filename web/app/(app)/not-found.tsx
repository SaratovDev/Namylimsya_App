import Link from "next/link";
import { SearchX } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";

export default function AppNotFound() {
  return (
    <div className="max-w-2xl mx-auto py-16 px-4">
      <div className="text-center space-y-4">
        <div className="flex justify-center">
          <SearchX className="h-16 w-16 text-muted-foreground" />
        </div>
        <h1 className="text-4xl font-bold">404</h1>
        <h2 className="text-xl font-semibold">Страница не найдена</h2>
        <p className="text-muted-foreground">
          Такой страницы нет, или она была удалена.
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