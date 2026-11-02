export const dynamic = "force-dynamic";

async function getHealth() {
  const base = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";
  try {
    const res = await fetch(`${base}/health`, { cache: "no-store" });
    if (!res.ok) return { status: "error", db: "unknown", redis: "unknown", code: res.status };
    return await res.json();
  } catch {
    return { status: "unreachable", db: "unknown", redis: "unknown" };
  }
}



export default async function Home() {
  const health = await getHealth();
  const ok = health.status === "ok";
  return (
    <main className="min-h-screen flex items-center justify-center p-8">
      <div className="max-w-md w-full rounded-2xl border p-6 space-y-4">
        <h1 className="text-2xl font-semibold">Намылимся — MVP</h1>
        <p className="text-sm opacity-70">Проверка связки Web → API → DB / Redis</p>
        <div className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span>API</span>
            <span className={ok ? "text-green-600" : "text-red-600"}>{health.status}</span>
          </div>
          <div className="flex justify-between">
            <span>Postgres</span>
            <span>{health.db}</span>
          </div>
          <div className="flex justify-between">
            <span>Redis</span>
            <span>{health.redis}</span>
          </div>
        </div>
        <p className="text-xs opacity-50">
          API: {process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1"}
        </p>
      </div>
    </main>
  );
}