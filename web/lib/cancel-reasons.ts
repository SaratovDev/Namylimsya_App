export type CancelReason = {
  value: string;
  label: string;
};

export const EVENT_CANCEL_REASONS: CancelReason[] = [
  { value: "plans_changed", label: "Планы изменились" },
  { value: "bad_weather", label: "Плохая погода" },
  { value: "health", label: "Проблемы со здоровьем" },
  { value: "venue", label: "Проблемы с местом проведения" },
  { value: "not_enough_people", label: "Недостаточно участников" },
  { value: "other", label: "Своя причина" },
];

export const APPLICATION_CANCEL_REASONS: CancelReason[] = [
  { value: "plans_changed", label: "Планы изменились" },
  { value: "health", label: "Плохое самочувствие" },
  { value: "work", label: "Работа / учёба" },
  { value: "family", label: "Семейные обстоятельства" },
  { value: "transport", label: "Не добраться до места" },
  { value: "other", label: "Своя причина" },
];

// Возвращает финальную строку для API.
// Если reason — «своя», возвращает пользовательский ввод.
export function resolveCancelReason(
  reasonValue: string,
  customText: string,
  reasons: CancelReason[]
): string {
  const found = reasons.find((r) => r.value === reasonValue);
  if (!found || found.value === "other") {
    return customText.trim();
  }
  return found.label;
}