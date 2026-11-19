"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { MailCheck } from "lucide-react";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

const ENABLED =
  process.env.NEXT_PUBLIC_EMAIL_VERIFICATION_ENABLED === "true";

export function EmailVerificationBanner() {
  const { accessToken, isReady } = useAuth();
  const [needsVerify, setNeedsVerify] = useState(false);
  const [resending, setResending] = useState(false);

  // Сброс при выходе — корректировка состояния при рендере.
  const [prevToken, setPrevToken] = useState(accessToken);
  if (prevToken !== accessToken) {
    setPrevToken(accessToken);
    if (!accessToken) setNeedsVerify(false);
  }

  useEffect(() => {
    if (!ENABLED || !isReady || !accessToken) return;
    let cancelled = false;
    apiFetch("/users/me", {}, accessToken)
      .then((res) => res.json())
      .then((data) => {
        if (!cancelled) setNeedsVerify(data.email_verified === false);
      })
      .catch(() => {
        // ignore
      });
    return () => {
      cancelled = true;
    };
  }, [accessToken, isReady]);

  if (!needsVerify) return null;

  const resend = async () => {
    if (!accessToken) return;
    setResending(true);
    try {
      await apiFetch(
        "/users/me/resend-verification",
        { method: "POST" },
        accessToken
      );
      toast.success("Письмо отправлено повторно");
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Не удалось отправить письмо"
      );
    } finally {
      setResending(false);
    }
  };

  return (
    <div className="border-b border-amber-500/30 bg-amber-500/10 px-4 py-2 text-sm flex items-center justify-center gap-3 flex-wrap">
      <MailCheck className="h-4 w-4 text-amber-600" />
      <span className="text-foreground/90">
        Подтвердите email — мы отправили ссылку на вашу почту.
      </span>
      <button
        onClick={resend}
        disabled={resending}
        className="text-primary hover:underline disabled:opacity-50"
      >
        {resending ? "Отправка…" : "Отправить повторно"}
      </button>
    </div>
  );
}
