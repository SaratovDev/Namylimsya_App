"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { apiFetch } from "@/lib/api";
import { AVATAR_PRESETS, avatarUrl } from "@/lib/avatars";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import { Check, ChevronDown, Loader2 } from "lucide-react";
import { UsernameHistoryEntry } from "@/lib/types";
import { INTERESTS, MAX_INTERESTS } from "@/lib/interests";

type Me = {
  id: string;
  email: string;
  username: string;
  gender: string | null;
  birth_date: string | null;
  show_age: boolean;
  city: string | null;
  bio: string | null;
  hobbies: string[] | null;
  telegram: string | null;
  vk: string | null;
  avatar_url: string | null;
  username_changed_at: string | null;
};

export default function SettingsPage() {
  const router = useRouter();
  const { accessToken, isReady } = useAuth();
  const [me, setMe] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [savingAvatar, setSavingAvatar] = useState<string | null>(null);

  // Поля формы
  const [username, setUsername] = useState("");
  const [gender, setGender] = useState<string>("");
  const [birthDate, setBirthDate] = useState("");
  const [city, setCity] = useState("");
  const [bio, setBio] = useState("");
  const [telegram, setTelegram] = useState("");
  const [vk, setVk] = useState("");
  const [showAge, setShowAge] = useState(true);
  const [hobbies, setHobbies] = useState<string[]>([]);
  // История ников
  const [historyOpen, setHistoryOpen] = useState(false);
  const [history, setHistory] = useState<UsernameHistoryEntry[]>([]);

  const loadProfile = useCallback(async () => {
    if (!isReady || !accessToken) return;
    setLoading(true);
    try {
      const res = await apiFetch("/users/me", {}, accessToken);
      const data: Me = await res.json();
      setMe(data);
      setUsername(data.username);
      setGender(data.gender ?? "");
      setBirthDate(data.birth_date ? data.birth_date.slice(0, 10) : "");
      setCity(data.city ?? "");
      setBio(data.bio ?? "");
      setTelegram(data.telegram ?? "");
      setVk(data.vk ?? "");
      setShowAge(data.show_age);
      setHobbies(data.hobbies ?? []);    } catch {
      toast.error("Не удалось загрузить профиль");
    } finally {
      setLoading(false);
    }
  }, [accessToken, isReady]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadProfile();
  }, [loadProfile]);

  const loadHistory = async () => {
    if (!me) return;
    try {
      const res = await apiFetch(`/users/${me.id}/username-history`, {}, null);
      const data = await res.json();
      setHistory(data.history ?? []);
    } catch {
      // silent
    }
  };

    const toggleHobby = (interest: string) => {
    setHobbies((prev) => {
      if (prev.includes(interest)) return prev.filter((h) => h !== interest);
      if (prev.length >= MAX_INTERESTS) return prev;
      return [...prev, interest];
    });
  };

  const toggleHistory = () => {
    if (!historyOpen) void loadHistory();
    setHistoryOpen((v) => !v);
  };

  const chooseAvatar = async (path: string) => {
    if (!me || !accessToken) return;
    setSavingAvatar(path);
    try {
      const res = await apiFetch(
        "/users/me",
        {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ avatar_url: path }),
        },
        accessToken
      );
      const updated = await res.json();
      setMe((prev) =>
        prev ? { ...prev, avatar_url: updated.avatar_url ?? path } : prev
      );
      toast.success("Аватар обновлён");
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Не удалось сохранить аватар"
      );
    } finally {
      setSavingAvatar(null);
    }
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!me || !accessToken) return;

    setSaving(true);
    try {

      const payload: Record<string, unknown> = {
        gender: gender || null,
        birth_date: birthDate || null,
        city,
        bio,
        telegram: telegram || null,
        vk: vk || null,
        show_age: showAge,
        hobbies,
      };

      if (username !== me.username) {
        payload.username = username;
      }

      const res = await apiFetch(
        "/users/me",
        {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
        accessToken
      );
      const updated = await res.json();
      setMe((prev) => (prev ? { ...prev, ...updated } : prev));
      toast.success("Профиль сохранён");
      router.refresh();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Не удалось сохранить"
      );
    } finally {
      setSaving(false);
    }
  };

  if (!isReady || loading) {
    return (
      <div className="max-w-2xl mx-auto py-6 px-4 space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-32 w-full rounded-xl" />
        <Skeleton className="h-64 w-full rounded-xl" />
      </div>
    );
  }

  if (!accessToken || !me) {
    return (
      <div className="max-w-2xl mx-auto py-6 px-4">
        <h1 className="text-3xl font-bold mb-6">Настройки</h1>
        <p className="text-muted-foreground">
          Войдите, чтобы редактировать профиль.
        </p>
      </div>
    );
  }

  const currentAvatar = avatarUrl(me.avatar_url);
  const canChangeUsername = !me.username_changed_at;

  return (
    <div className="max-w-2xl mx-auto py-6 px-4">
      <h1 className="text-3xl font-bold mb-6">Настройки</h1>

      {/* ─── Аватар ──────────────────────────────────────────────── */}
      <section className="border border-border rounded-xl p-6 bg-card mb-6">
        <h2 className="text-lg font-semibold mb-4">Аватар</h2>
        <div className="flex items-center gap-4 mb-6">
          <Avatar className="h-16 w-16">
            <AvatarImage src={currentAvatar ?? ""} alt={me.username} />
            <AvatarFallback className="bg-primary text-primary-foreground text-xl">
              {me.username?.[0]?.toUpperCase() ?? "?"}
            </AvatarFallback>
          </Avatar>
          <div>
            <p className="text-sm font-medium">{me.username}</p>
            <p className="text-xs text-muted-foreground">
              Выберите из готовых или оставьте инициал
            </p>
          </div>
        </div>

        <Label className="mb-2 block">Готовые аватары</Label>
        <div className="grid grid-cols-4 sm:grid-cols-6 gap-3">
          <button
            type="button"
            onClick={() => chooseAvatar("")}
            className={cn(
              "relative aspect-square rounded-full border-2 flex items-center justify-center bg-muted text-muted-foreground text-sm font-medium hover:border-primary/50 transition-colors",
              !me.avatar_url ? "border-primary" : "border-transparent"
            )}
            title="Без аватара"
            disabled={savingAvatar !== null}
          >
            {me.username?.[0]?.toUpperCase() ?? "?"}
            {!me.avatar_url && (
              <span className="absolute -bottom-1 -right-1 h-5 w-5 rounded-full bg-primary text-primary-foreground flex items-center justify-center">
                <Check className="h-3 w-3" />
              </span>
            )}
          </button>

          {AVATAR_PRESETS.map((path) => {
            const selected = me.avatar_url === path;
            const isSaving = savingAvatar === path;
            return (
              <button
                key={path}
                type="button"
                onClick={() => chooseAvatar(path)}
                className={cn(
                  "relative aspect-square rounded-full overflow-hidden border-2 transition-all hover:scale-105",
                  selected ? "border-primary" : "border-transparent",
                  isSaving && "opacity-50"
                )}
                title="Выбрать этот аватар"
                disabled={savingAvatar !== null}
              >
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={path} alt="" className="w-full h-full object-cover" />
                {selected && (
                  <span className="absolute bottom-0 right-0 h-5 w-5 rounded-full bg-primary text-primary-foreground flex items-center justify-center">
                    <Check className="h-3 w-3" />
                  </span>
                )}
              </button>
            );
          })}
        </div>
      </section>

      {/* ─── Профиль ─────────────────────────────────────────────── */}
      <form onSubmit={handleSave} className="space-y-6">
        <section className="border border-border rounded-xl p-6 bg-card">
          <h2 className="text-lg font-semibold mb-4">Профиль</h2>

          <div className="space-y-4">
            {/* Username */}
            <div className="space-y-2">
              <Label htmlFor="username">Имя пользователя</Label>
              <Input
                id="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                disabled={!canChangeUsername}
                maxLength={30}
              />
              {!canChangeUsername ? (
                <button
                  type="button"
                  onClick={toggleHistory}
                  className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
                >
                  <ChevronDown
                    className={cn(
                      "h-3 w-3 transition-transform",
                      historyOpen && "rotate-180"
                    )}
                  />
                  История смены имён (изменить можно один раз)
                </button>
              ) : (
                <p className="text-xs text-muted-foreground">
                  Можно сменить один раз за всё время. После — только история.
                </p>
              )}

              {historyOpen && (
                <div className="mt-2 p-3 rounded-md bg-muted/30 border border-border/60 space-y-1">
                  {history.length === 0 ? (
                    <p className="text-xs text-muted-foreground">
                      Имя не менялось.
                    </p>
                  ) : (
                    history.map((h) => (
                      <div
                        key={h.id}
                        className="flex items-center justify-between text-xs"
                      >
                        <span className="font-mono">@{h.old_username}</span>
                        <span className="text-muted-foreground">
                          {new Date(h.changed_at).toLocaleDateString("ru-RU")}
                        </span>
                      </div>
                    ))
                  )}
                </div>
              )}
            </div>

            {/* Gender */}
            <div className="space-y-2">
              <Label htmlFor="gender">Пол</Label>
              <select
                id="gender"
                value={gender}
                onChange={(e) => setGender(e.target.value)}
                className="flex h-9 w-full rounded-md border border-input bg-input px-3 py-2 text-sm"
              >
                <option value="">Не указан</option>
                <option value="male">Мужской</option>
                <option value="female">Женский</option>
                <option value="other">Другой</option>
              </select>
            </div>

            {/* Birth date + show_age */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="birth_date">Дата рождения</Label>
                <Input
                  id="birth_date"
                  type="date"
                  value={birthDate}
                  onChange={(e) => setBirthDate(e.target.value)}
                  max={new Date().toISOString().slice(0, 10)}
                />
              </div>
              <div className="flex items-center gap-2 mt-6">
                <input
                  type="checkbox"
                  id="show_age"
                  checked={showAge}
                  onChange={(e) => setShowAge(e.target.checked)}
                  className="h-4 w-4 rounded border-gray-300 text-primary focus:ring-primary"
                />
                <Label htmlFor="show_age" className="cursor-pointer">
                  Показывать возраст в профиле
                </Label>
              </div>
            </div>

            {/* City */}
            <div className="space-y-2">
              <Label htmlFor="city">Город</Label>
              <Input
                id="city"
                value={city}
                onChange={(e) => setCity(e.target.value)}
                placeholder="Саратов"
              />
            </div>

            {/* Bio */}
            <div className="space-y-2">
              <Label htmlFor="bio">О себе</Label>
              <Textarea
                id="bio"
                value={bio}
                onChange={(e) => setBio(e.target.value)}
                placeholder="Пара слов о себе, что любишь, куда ходишь…"
                rows={4}
                maxLength={500}
              />
              <p className="text-xs text-muted-foreground text-right">
                {bio.length}/500
              </p>
            </div>

            {/* Interests */}
            <div className="space-y-2">
              <Label>
                Интересы{" "}
                <span className="text-xs text-muted-foreground">
                  (выбрано {hobbies.length} из {MAX_INTERESTS})
                </span>
              </Label>
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-1.5">
                {INTERESTS.map((interest) => {
                  const selected = hobbies.includes(interest);
                  const disabled =
                    !selected && hobbies.length >= MAX_INTERESTS;
                  return (
                    <button
                      key={interest}
                      type="button"
                      onClick={() => toggleHobby(interest)}
                      disabled={disabled}
                      className={cn(
                        "text-xs px-2.5 py-1.5 rounded-full border text-left transition-colors",
                        selected
                          ? "bg-primary/15 border-primary text-primary font-medium"
                          : "border-border text-muted-foreground hover:bg-muted",
                        disabled && "opacity-40 cursor-not-allowed"
                      )}
                    >
                      {interest}
                    </button>
                  );
                })}
              </div>
              {hobbies.length >= MAX_INTERESTS && (
                <p className="text-xs text-muted-foreground">
                  Максимум {MAX_INTERESTS}. Снимите один, чтобы выбрать другой.
                </p>
              )}
            </div>

            {/* Telegram + VK */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="telegram">Telegram</Label>
                <Input
                  id="telegram"
                  value={telegram}
                  onChange={(e) => setTelegram(e.target.value)}
                  placeholder="@username"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="vk">VK</Label>
                <Input
                  id="vk"
                  value={vk}
                  onChange={(e) => setVk(e.target.value)}
                  placeholder="vk.com/username"
                />
              </div>
            </div>
          </div>
        </section>

        {/* Кнопка сохранения */}
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="ghost"
            onClick={() => router.push(`/users/${me.id}`)}
          >
            Посмотреть профиль
          </Button>
          <Button type="submit" disabled={saving} className="gap-2">
            {saving ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" />
                Сохранение…
              </>
            ) : (
              "Сохранить"
            )}
          </Button>
        </div>
      </form>
    </div>
  );
}