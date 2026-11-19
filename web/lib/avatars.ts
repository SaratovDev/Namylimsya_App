// Пресеты аватаров. Файлы лежат в web/public/avatars/.
// Позже можно добавить загрузку своих — avatar_url будет либо /avatars/…,
// либо https://s3.example.com/…, либо null.

export const AVATAR_PRESETS = [
  "/avatars/preset-01.svg",
  "/avatars/preset-02.svg",
  "/avatars/preset-03.svg",
  "/avatars/preset-04.svg",
  "/avatars/preset-05.svg",
  "/avatars/preset-06.svg",
  "/avatars/preset-07.svg",
  "/avatars/preset-08.svg",
  "/avatars/preset-09.svg",
  "/avatars/preset-10.svg",
  "/avatars/preset-11.svg",
  "/avatars/preset-12.svg",
] as const;

export type AvatarPreset = (typeof AVATAR_PRESETS)[number];

// Возвращает URL, готовый для <img src> или <AvatarImage src>.
// null / undefined → null (UI покажет fallback с инициалом).
export function avatarUrl(path: string | null | undefined): string | null {
  if (!path) return null;
  // Относительные пути (/avatars/…) и полные URL (https://s3/…) отдаём как есть.
  if (path.startsWith("/") || path.startsWith("http://") || path.startsWith("https://")) {
    return path;
  }
  // На всякий случай: если пришло что-то странное, не подставляем.
  return null;
}