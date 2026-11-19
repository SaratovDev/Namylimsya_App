"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { MapPicker } from "@/components/map-picker";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { CATEGORIES } from "@/lib/categories";

export function CreateEventDialog({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { accessToken } = useAuth();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  // Общие поля
  const [title, setTitle] = useState("");
  const [category, setCategory] = useState(CATEGORIES[0].slug);
  const [description, setDescription] = useState("");
  const [startsAt, setStartsAt] = useState("");
  const [limit, setLimit] = useState("");
  const [hideExact, setHideExact] = useState(false);

  // Локация — с карты
  const [lat, setLat] = useState(0);
  const [lng, setLng] = useState(0);
  const [city, setCity] = useState("");
  const [address, setAddress] = useState("");

  const hasLocation = lat !== 0 && lng !== 0 && city !== "";

  const reset = () => {
    setTitle("");
    setCategory(CATEGORIES[0].slug);
    setDescription("");
    setStartsAt("");
    setLimit("");
    setHideExact(false);
    setLat(0);
    setLng(0);
    setCity("");
    setAddress("");
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!accessToken) {
      toast.error("Войдите, чтобы создать событие");
      return;
    }
    if (!hasLocation) {
      toast.error("Укажите место на карте");
      return;
    }

    setLoading(true);
    try {
      const isoDate = new Date(startsAt).toISOString();
      const payload = {
        title,
        category,
        description,
        starts_at: isoDate,
        city,
        address,
        lat,
        lng,
        hide_exact_location: hideExact,
        participant_limit: limit ? parseInt(limit, 10) : undefined,
      };
      await apiFetch(
        "/events",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
        accessToken
      );
      toast.success("Событие создано!");
      setOpen(false);
      reset();
      router.refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Ошибка создания");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        setOpen(v);
        if (!v) reset();
      }}
    >
      <DialogTrigger className={buttonVariants({ className: "gap-2" })}>
        {children}
      </DialogTrigger>

      <DialogContent className="md:max-w-4xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogDescription className="sr-only">
            Заполните форму и укажите место на карте.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="mt-2">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* ─── ЛЕВАЯ КОЛОНКА: контент события ─────────────────── */}
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="title">Название</Label>
                <Input
                  id="title"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder="Например: Утренняя прогулка по набережной"
                  required
                  maxLength={100}
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-2">
                  <Label htmlFor="category">Категория</Label>
                  <select
                    id="category"
                    className="flex h-9 w-full rounded-md border border-input bg-input px-3 py-2 text-sm"
                    value={category}
                    onChange={(e) => setCategory(e.target.value)}
                  >
                    {CATEGORIES.map((c) => (
                      <option key={c.slug} value={c.slug}>
                        {c.label}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="starts_at">Дата и время</Label>
                  <Input
                    id="starts_at"
                    type="datetime-local"
                    value={startsAt}
                    onChange={(e) => setStartsAt(e.target.value)}
                    required
                  />
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor="description">Описание</Label>
                <Textarea
                  id="description"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="Что будем делать? Что взять с собой?"
                  rows={3}
                />
              </div>

              <div className="space-y-2">
                <Label>Место на карте</Label>
                <MapPicker
                  city={city}
                  address={address}
                  lat={lat}
                  lng={lng}
                  onChange={(d) => {
                    setCity(d.city);
                    setAddress(d.address);
                    setLat(d.lat);
                    setLng(d.lng);
                  }}
                />
              </div>
            </div>

            {/* ─── ПРАВАЯ КОЛОНКА: адрес, лимит, кнопки ───────────── */}
            <div className="flex flex-col">
              <div className="space-y-4 flex-1">
                <div className="space-y-2">
                  <Label>Город</Label>
                  <div className="h-9 px-3 flex items-center rounded-md border bg-muted/30 text-sm">
                    {city || (
                      <span className="text-muted-foreground">
                        Определится по карте
                      </span>
                    )}
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="address">Адрес</Label>
                  <Input
                    id="address"
                    value={address}
                    onChange={(e) =>
                      setAddress(e.target.value)
                    }
                    placeholder="Улица, дом"
                    disabled={!hasLocation}
                  />
                  <p className="text-xs text-muted-foreground">
                    Заполняется с карты. Можно уточнить вручную
                    (например, «корпус 2»).
                  </p>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="limit">Лимит участников</Label>
                  <Input
                    id="limit"
                    type="number"
                    min="1"
                    value={limit}
                    onChange={(e) => setLimit(e.target.value)}
                    placeholder="Без ограничений"
                  />
                </div>

                <div className="flex items-start gap-2 pt-2">
                  <input
                    type="checkbox"
                    id="hideExact"
                    checked={hideExact}
                    onChange={(e) => setHideExact(e.target.checked)}
                    className="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary focus:ring-primary"
                  />
                  <div className="flex-1">
                    <Label htmlFor="hideExact" className="cursor-pointer">
                      Скрыть точное место
                    </Label>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      Точный адрес увидят только одобренные участники.
                      Остальные — только район.
                    </p>
                  </div>
                </div>
              </div>

              {/* Кнопки — прижаты к низу правой колонки */}
              <div className="flex justify-end gap-2 pt-4 mt-4 border-t md:mt-auto md:pt-6">
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => setOpen(false)}
                  disabled={loading}
                >
                  Отмена
                </Button>
                <Button
                  type="submit"
                  disabled={loading || !startsAt || !hasLocation}
                >
                  {loading ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      Создание…
                    </>
                  ) : (
                    "Создать событие"
                  )}
                </Button>
              </div>
            </div>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}