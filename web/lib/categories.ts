export type Category = {
  slug: string;
  label: string;
};

export const CATEGORIES: Category[] = [
  { slug: "прогулки",           label: "Прогулки" },
  { slug: "спорт",              label: "Спорт" },
  { slug: "напитки/еда",        label: "Напитки/еда" },
  { slug: "животные",           label: "Животные" },
  { slug: "культура",           label: "Культура" },
  { slug: "вечеринки",          label: "Вечеринки" },
  { slug: "путешествия",        label: "Путешествия" },
  { slug: "обучение",           label: "Обучение" },
  { slug: "волонтерство",       label: "Волонтёрство" },
  { slug: "романтика/свидания", label: "Романтика/свидания" },
  { slug: "настолки/игры",      label: "Настолки/игры" },
  { slug: "другое",             label: "Другое" },
];

const labelBySlug = new Map(CATEGORIES.map((c) => [c.slug, c.label]));

export function categoryLabel(slug: string | null | undefined): string {
  if (!slug) return "";
  // Fallback: если в БД лежит невалидное (до миграции) — вернём как есть,
  // чтобы UI не показывал пустоту.
  return labelBySlug.get(slug) ?? slug;
}

export function isValidCategorySlug(slug: string): boolean {
  return labelBySlug.has(slug);
}