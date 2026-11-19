"use client";

import { useState } from "react";
import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { toast } from "sonner";
import { apiFetch } from "@/lib/api";
import { LEGAL_ENTITY } from "@/lib/legal";

export default function RegisterPage() {
  const { login } = useAuth();
  const [form, setForm] = useState({ email: "", password: "", username: "", city: "" });
  const [acceptedTerms, setAcceptedTerms] = useState(false);
  const [ageConfirmed, setAgeConfirmed] = useState(false);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!ageConfirmed) {
      toast.error("Подтвердите, что вам 18 лет или больше");
      return;
    }
    if (!acceptedTerms) {
      toast.error("Необходимо согласие на обработку персональных данных");
      return;
    }
    setLoading(true);
    try {
      await apiFetch("/auth/register", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          ...form,
          consent_accepted: true,
          age_confirmed: true,
          consent_version: LEGAL_ENTITY.consentVersion,
        }),
      }, null);
      await login(form.email, form.password);
      toast.success("Аккаунт создан!");
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "Ошибка регистрации";
      toast.error(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card className="w-full">
      <CardHeader className="text-center">
        <CardTitle className="text-2xl">Регистрация</CardTitle>
        <CardDescription>Создайте аккаунт, чтобы создавать события и подавать заявки</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="email">Email</Label>
            <Input id="email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="username">Имя пользователя</Label>
            <Input id="username" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="city">Город</Label>
            <Input id="city" value={form.city} onChange={(e) => setForm({ ...form, city: e.target.value })} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="password">Пароль</Label>
            <Input id="password" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required minLength={8} />
            <p className="text-xs text-muted-foreground">Минимум 8 символов</p>
          </div>

<div className="flex items-start gap-3 pt-2">
  <input
    type="checkbox"
    id="ageConfirmed"
    checked={ageConfirmed}
    onChange={(e) => setAgeConfirmed(e.target.checked)}
    className="mt-1 h-4 w-4 shrink-0 rounded border-border accent-primary cursor-pointer"
    required
  />
  <Label
    htmlFor="ageConfirmed"
    className="text-sm leading-snug cursor-pointer font-normal !block text-left"
  >
    Подтверждаю, что мне 18 лет или больше
  </Label>
</div>

<div className="flex items-start gap-3">
  <input
    type="checkbox"
    id="acceptTerms"
    checked={acceptedTerms}
    onChange={(e) => setAcceptedTerms(e.target.checked)}
    className="mt-1 h-4 w-4 shrink-0 rounded border-border accent-primary cursor-pointer"
    required
  />
  <Label
    htmlFor="acceptTerms"
    className="text-sm leading-snug cursor-pointer font-normal !block text-left"
  >
    Я согласен с{" "}
    <Link href="/privacy" target="_blank" className="text-primary hover:underline">
      политикой конфиденциальности
    </Link>{" "}
    и{" "}
    <Link href="/terms" target="_blank" className="text-primary hover:underline">
      пользовательским соглашением
    </Link>
  </Label>
</div>

<Button type="submit" className="w-full" disabled={loading || !acceptedTerms || !ageConfirmed}>
  {loading ? "Создание..." : "Зарегистрироваться"}
</Button>
        </form>
      </CardContent>
      <CardFooter className="justify-center text-sm text-muted-foreground">
        Уже есть аккаунт?{" "}
        <Link href="/login" className="text-primary hover:underline ml-1">
          Войти
        </Link>
      </CardFooter>
    </Card>
  );
}