export type JwtPayload = {
  user_id?: string;
  role?: string;
  exp?: number;
  iat?: number;
  jti?: string;
  [key: string]: unknown;
};

// atob падает на base64url (- и _) и без паддинга. Декодируем вручную.
export function decodeJwt(token: string): JwtPayload | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const b64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = b64 + "=".repeat((4 - (b64.length % 4)) % 4);
    return JSON.parse(atob(padded)) as JwtPayload;
  } catch {
    return null;
  }
}

export function getUserIdFromToken(
  token: string | null | undefined
): string | null {
  if (!token) return null;
  return decodeJwt(token)?.user_id ?? null;
}

// true, если exp отсутствует, уже прошёл, или истекает в ближайшие thresholdSec.
export function isTokenExpiringSoon(token: string, thresholdSec = 60): boolean {
  const payload = decodeJwt(token);
  if (!payload?.exp) return true;
  return Date.now() / 1000 > payload.exp - thresholdSec;
}