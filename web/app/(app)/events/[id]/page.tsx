import { Suspense } from "react";
import { EventContent } from "@/components/event-content";
import { Skeleton } from "@/components/ui/skeleton";

type Props = {
  params: Promise<{ id: string }>;
};

function EventSkeleton() {
  return (
    <div className="space-y-4">
      <Skeleton className="h-12 w-3/4" />
      <Skeleton className="h-6 w-1/3" />
      <Skeleton className="h-64 w-full rounded-xl" />
    </div>
  );
}

async function EventPageWrapper({ params }: Props) {
  const { id } = await params;
  return <EventContent id={id} />;
}

export default function EventPage({ params }: Props) {
  return (
    <div className="py-4 sm:py-6 max-w-5xl mx-auto px-2 sm:px-4">
      <Suspense fallback={<EventSkeleton />}>
        <EventPageWrapper params={params} />
      </Suspense>
    </div>
  );
}