"use client";

import { Suspense } from "react";
import Link from "next/link";
import { AdminNav, AdminNavSkeleton } from "@/components/admin-nav";

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen bg-background flex flex-col">
      <header className="h-16 border-b border-border bg-brand-black text-brand-cream flex items-center px-3 md:px-6 gap-3">
        <Link href="/admin" className="font-bold text-lg shrink-0">
          Намылимся<span className="text-primary">.</span>{" "}
          <span className="text-xs opacity-70">admin</span>
        </Link>

        <Suspense fallback={<AdminNavSkeleton />}>
          <AdminNav />
        </Suspense>
      </header>

      <main className="flex-1 p-4 md:p-6">{children}</main>
    </div>
  );
}