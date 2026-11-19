"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { decodeJwt } from "@/lib/jwt";
import { Skeleton } from "@/components/ui/skeleton";

export default function ProfileRedirectPage() {
  const router = useRouter();
  const { accessToken, isReady } = useAuth();

  useEffect(() => {
    if (!isReady) return;
    const me = accessToken ? decodeJwt(accessToken)?.user_id : null;
    if (me) {
      router.replace(`/users/${me}`);
    } else {
      router.replace("/login");
    }
  }, [isReady, accessToken, router]);

  return (
    <div className="max-w-3xl mx-auto py-6 px-4 space-y-4">
      <Skeleton className="h-8 w-48" />
      <Skeleton className="h-48 w-full rounded-xl" />
    </div>
  );
}