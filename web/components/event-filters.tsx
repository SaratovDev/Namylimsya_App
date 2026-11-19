"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useState, useEffect } from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { X, CalendarIcon } from "lucide-react";
import { CATEGORIES } from "@/lib/categories";

type EventFiltersProps = {
  onApply?: () => void;
  /** Куда пушить при apply/clear. По умолчанию "/". Для архива — "/archive". */
  basePath?: string;
};

export function EventFilters({ onApply, basePath = "/" }: EventFiltersProps = {}) {
  const router = useRouter();
  const searchParams = useSearchParams();

  // URL — источник правды
  const urlCategories = searchParams.get("categories") || "";
  const urlCity = searchParams.get("city") || "";
  const urlDateFrom = searchParams.get("date_from") || "";
  const urlDateTo = searchParams.get("date_to") || "";

  // Локальный стейт (черновик, пока не нажали «Применить»)
  const [cityInput, setCityInput] = useState(urlCity);
  const [localCategories, setLocalCategories] = useState<string[]>(
    urlCategories
      .split(",")
      .map((s) => s.trim().toLowerCase())
      .filter(Boolean)
  );
  const [dateFrom, setDateFrom] = useState(urlDateFrom);
  const [dateTo, setDateTo] = useState(urlDateTo);

  // Синхронизация при изменении URL (напр., кнопка «Назад»)
  // Локальный стейт — «черновик», URL — источник правды. Здесь синхронизация безопасна.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setCityInput(urlCity);
    setLocalCategories(
      urlCategories.split(",").map((s) => s.trim().toLowerCase()).filter(Boolean)
    );
    setDateFrom(urlDateFrom ? urlDateFrom.slice(0, 10) : "");
    setDateTo(urlDateTo ? urlDateTo.slice(0, 10) : "");
  }, [urlCity, urlCategories, urlDateFrom, urlDateTo]);

  const toRFC3339 = (dateStr: string, endOfDay = false): string => {
    if (!dateStr) return "";
    // Если уже ISO (содержит "T") — возвращаем как есть
    if (dateStr.includes("T")) return dateStr;
    // Иначе YYYY-MM-DD → RFC3339
    const time = endOfDay ? "T23:59:59Z" : "T00:00:00Z";
    return new Date(dateStr + time).toISOString();
  };

  const applyFilters = () => {
    const params = new URLSearchParams();
    if (cityInput.trim()) params.set("city", cityInput.trim());
    if (localCategories.length > 0) params.set("categories", localCategories.join(","));

    const from = toRFC3339(dateFrom, false);
    if (from) params.set("date_from", from);

    const to = toRFC3339(dateTo, true);
    if (to) params.set("date_to", to);

    const qs = params.toString();
    router.push(qs ? `${basePath}?${qs}` : basePath);
    onApply?.();
  };

  const toggleCategory = (cat: string) => {
    setLocalCategories((prev) =>
      prev.includes(cat) ? prev.filter((c) => c !== cat) : [...prev, cat]
    );
  };

  const clearAll = () => {
    setCityInput("");
    setLocalCategories([]);
    setDateFrom("");
    setDateTo("");
    router.push(basePath);
    onApply?.();
  };

  const hasFilters =
    urlCity || localCategories.length > 0 || urlDateFrom || urlDateTo;

  return (
    <div className="space-y-6 lg:sticky lg:top-20">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">Фильтры</h2>
        {hasFilters && (
          <Button
            variant="ghost"
            size="sm"
            className="h-8 text-muted-foreground hover:text-foreground"
            onClick={clearAll}
          >
            <X className="h-3 w-3 mr-1" />
            Сбросить
          </Button>
        )}
      </div>

      <div className="space-y-3">
        <Label>Категории</Label>
        <div className="flex flex-wrap gap-1.5">
          {CATEGORIES.map((cat) => (
            <Button
              key={cat.slug}
              variant={localCategories.includes(cat.slug) ? "default" : "outline"}
              size="sm"
              className="text-xs h-7 rounded-full px-3"
              onClick={() => toggleCategory(cat.slug)}
            >
              {cat.label}
            </Button>
          ))}
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="city">Город</Label>
        <Input
          id="city"
          placeholder="Например, Саратов"
          value={cityInput}
          onChange={(e) => setCityInput(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && applyFilters()}
        />
      </div>

      <div className="space-y-2">
        <Label>Даты</Label>
        <div className="space-y-2">
          <div className="relative">
            <Input
              type="date"
              value={dateFrom}
              onChange={(e) => setDateFrom(e.target.value)}
              className="pl-9"
            />
            <CalendarIcon className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
          </div>
          <div className="relative">
            <Input
              type="date"
              value={dateTo}
              onChange={(e) => setDateTo(e.target.value)}
              className="pl-9"
            />
            <CalendarIcon className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
          </div>
        </div>
      </div>

      <Button className="w-full" onClick={applyFilters}>
        Применить
      </Button>
    </div>
  );
}