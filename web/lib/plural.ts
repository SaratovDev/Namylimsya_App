// pluralRu — русские склонения: 1 день, 2 дня, 5 дней.
// Пример: pluralRu(n, "день", "дня", "дней").
// Учитывает 11–14 (всегда "дней"), 21 (день), 22 (дня) и т.д.
export function pluralRu(
  n: number,
  one: string,
  few: string,
  many: string
): string {
  const abs = Math.abs(n) % 100;
  if (abs >= 11 && abs <= 19) return many;
  const last = abs % 10;
  if (last === 1) return one;
  if (last >= 2 && last <= 4) return few;
  return many;
}