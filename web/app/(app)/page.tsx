"use client";

import { Suspense, useState } from "react";
import { SlidersHorizontal } from "lucide-react";
import { EventFeed } from "@/components/event-feed";
import { EventFilters } from "@/components/event-filters";
import { Skeleton } from "@/components/ui/skeleton";
import { Sheet, SheetContent, SheetTitle, SheetTrigger } from "@/components/ui/sheet";

function FeedSkeleton() {
  return (
    <div className="space-y-4">
      {[1, 2, 3].map((i) => (
        <Skeleton key={i} className="h-56 w-full rounded-xl" />
      ))}
    </div>
  );
}

export default function HomePage() {
  const [filtersOpen, setFiltersOpen] = useState(false);

  return (
    <div className="flex gap-6 py-6">
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between mb-6 gap-2">
          <h1 className="text-3xl font-bold">Лента событий</h1>

          {/* Кнопка «Фильтры» — только на мобильных/планшетах */}
          <Sheet open={filtersOpen} onOpenChange={setFiltersOpen}>
            <SheetTrigger
              className="lg:hidden inline-flex items-center gap-2 h-9 px-3 rounded-md border border-border bg-background hover:bg-muted text-sm font-medium transition-colors"
            >
              <SlidersHorizontal className="h-4 w-4" />
              Фильтры
            </SheetTrigger>
            <SheetContent side="right" className="w-[85vw] max-w-[360px] sm:w-[360px] overflow-y-auto p-4">
              <SheetTitle className="sr-only">Фильтры событий</SheetTitle>
              <Suspense fallback={<div className="text-sm text-muted-foreground">Загрузка фильтров…</div>}>
                <EventFilters onApply={() => setFiltersOpen(false)} />
              </Suspense>            </SheetContent>
          </Sheet>
        </div>

        <Suspense fallback={<FeedSkeleton />}>
          <EventFeed />
        </Suspense>
      </div>
      <aside className="hidden lg:block w-[280px] shrink-0">
        <Suspense fallback={null}>
          <EventFilters />
        </Suspense>
      </aside>
    </div>
  );
}