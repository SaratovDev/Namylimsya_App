import { Event } from "./types";

export type StatusBadge = {
  text: string;
  variant: "default" | "destructive" | "secondary" | "warning";
};



// Дефолтная длительность события, если нет ends_at (согласовано с
// EventWorker и SQL архива: `starts_at + interval '2 hours'`).
const EVENT_DEFAULT_DURATION_MS = 2 * 60 * 60 * 1000;

// Бейдж статуса: «Отменено» / «Завершено» / «Идёт сейчас» / «Скоро» /
// «Через N ч.» / null.
export function getEventStatusBadge(event: Event): StatusBadge | null {
  if (event.status === "cancelled") {
    return { text: "Отменено", variant: "destructive" };
  }
  if (event.status === "completed") {
    return { text: "Завершено", variant: "secondary" };
  }

  const now = new Date();
  const start = new Date(event.starts_at);
  const diffMs = start.getTime() - now.getTime();
  const diffHours = diffMs / (1000 * 60 * 60);
  const diffDays = diffHours / 24;

  // Идёт сейчас: старт в прошлом, но не прошло ещё 2 часа.
  // До появления ends_at — эвристика по дефолтной длительности.
  if (diffMs <= 0 && diffMs > -EVENT_DEFAULT_DURATION_MS) {
    return { text: "Идёт сейчас", variant: "default" };
  }

  
  // Уже прошло (EventWorker ещё не перевёл в completed — бейдж не показываем).
  if (diffMs < 0) return null;

if (diffHours < 3) {
  const hours = Math.ceil(diffHours);
  return { text: `Через ${hours} ч.`, variant: "warning" };
}
if (diffDays < 3) {
  return { text: "Скоро", variant: "warning" };
}
  return null;
}

// Формат даты: "15 января, ср, 21:00"
export function formatEventDate(iso: string): string {
  const d = new Date(iso);
  const dateStr = d.toLocaleDateString("ru-RU", {
    day: "numeric",
    month: "long",
    weekday: "short",
  });
  const timeStr = d.toLocaleTimeString("ru-RU", {
    hour: "2-digit",
    minute: "2-digit",
  });
  return `${dateStr}, ${timeStr}`;
}

// Прогресс участников в процентах
export function getParticipantsProgress(event: Event): number {
  if (!event.participant_limit) return 0; // нет лимита — 0%
  return Math.min(
    100,
    Math.round((event.participants_count / event.participant_limit) * 100)
  );
}