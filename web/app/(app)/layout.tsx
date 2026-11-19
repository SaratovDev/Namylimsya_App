import { Suspense } from "react";
import { AppSidebar } from "@/components/app-sidebar";
import { AppHeader } from "@/components/app-header";
import { EmailVerificationBanner } from "@/components/email-verification-banner";

function SidebarSkeleton() {
  return <div className="hidden md:block w-[260px] border-r border-border bg-card" />;
}

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen">
      {/* Десктопный сайдбар. На мобильных скрыт — доступен через гамбургер в AppHeader. */}
      <div className="hidden md:flex">
        <Suspense fallback={<SidebarSkeleton />}>
          <AppSidebar />
        </Suspense>
      </div>
      <div className="flex-1 flex flex-col min-w-0">
        <AppHeader />
        <EmailVerificationBanner />
        <main className="flex-1 px-4 md:px-12">{children}</main>
      </div>
    </div>
  );
}