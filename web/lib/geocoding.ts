// Geocoding: Photon (komoot) + Nominatim (fallback).
// Photon — быстрый, CORS разрешён, данные OSM, локализация RU.
// Nominatim — медленнее, rate-limit 1 req/sec, но стабильный fallback.

export type GeocodedPlace = {
  lat: number;
  lng: number;
  /** Город (без области): «Саратов». */
  city: string;
  /** Полный адрес: «Московская улица, 15 (Саратовская область)». */
  address: string;
  /** Человеко-читаемое для dropdown поиска: «Московская улица, 15, Саратов». */
  displayName: string;
};

const PHOTON_BASE = "https://photon.komoot.io";
const NOMINATIM_BASE = "https://nominatim.openstreetmap.org";

const NOMINATIM_HEADERS = {
  // Nominatim требует User-Agent с идентификацией приложения.
  "User-Agent": "Namylimsya/1.0 (https://namylimsya.ru)",
  Accept: "application/json",
};

// ─── Photon response shape ────────────────────────────────────────────

type PhotonFeature = {
  geometry: { coordinates: [number, number] }; // [lng, lat]
  properties: {
    osm_id?: number;
    osm_type?: string;
    street?: string;
    housenumber?: string;
    city?: string;
    town?: string;
    village?: string;
    county?: string;
    state?: string;
    district?: string;
    country?: string;
    name?: string;
    postcode?: string;
  };
};

type PhotonResponse = {
  features: PhotonFeature[];
};

// ─── Address formatting ───────────────────────────────────────────────

// formatAddress: «улица, дом (область)» — область в скобках в конце,
// чтобы не было путаницы с одноимёнными населёнными пунктами.
// Fallbacks:
//   есть street + housenumber → «street, housenumber (state)»
//   есть street              → «street (state)»
//   есть city                → «city (state)»
//   есть state               → «state»
function formatAddress(p: PhotonFeature["properties"]): string {
  const parts: string[] = [];

  const streetLine = [p.street, p.housenumber].filter(Boolean).join(", ");
  if (streetLine) {
    parts.push(streetLine);
  } else {
    const locality = p.city || p.town || p.village || p.name;
    if (locality) parts.push(locality);
  }

  const state = p.state;
  if (state) {
    parts.push(`(${state})`);
  }

  return parts.join(" ").trim();
}

// extractCity: только город/посёлок, без области.
function extractCity(p: PhotonFeature["properties"]): string {
  return p.city || p.town || p.village || p.name || "";
}

// displayName: для dropdown поиска — «улица, дом, город».
function buildDisplayName(p: PhotonFeature["properties"]): string {
  const parts: string[] = [];
  if (p.street) parts.push(p.street);
  if (p.housenumber) parts.push(p.housenumber);
  const city = extractCity(p);
  if (city && city !== p.street) parts.push(city);
  if (parts.length === 0 && p.name) parts.push(p.name);
  if (parts.length === 0 && p.state) parts.push(p.state);
  return parts.join(", ");
}

function photonFeatureToPlace(f: PhotonFeature): GeocodedPlace {
  return {
    lat: f.geometry.coordinates[1],
    lng: f.geometry.coordinates[0],
    city: extractCity(f.properties),
    address: formatAddress(f.properties),
    displayName: buildDisplayName(f.properties) || "Без названия",
  };
}

// ─── Public API ───────────────────────────────────────────────────────

// reverseGeocode: lat/lng → адрес. Photon первым, Nominatim — fallback.
// Возвращает null, если оба провалились.
export async function reverseGeocode(
  lat: number,
  lng: number
): Promise<GeocodedPlace | null> {
  // Photon
  try {
    const url = `${PHOTON_BASE}/reverse?lat=${lat}&lon=${lng}&lang=ru&limit=1`;
    const res = await fetch(url);
    if (res.ok) {
      const data: PhotonResponse = await res.json();
      if (data.features && data.features.length > 0) {
        return photonFeatureToPlace(data.features[0]);
      }
    }
  } catch {
    // network — идём в fallback
  }

  // Nominatim fallback
  try {
    const url = `${NOMINATIM_BASE}/reverse?lat=${lat}&lon=${lng}&format=json&accept-language=ru&zoom=18`;
    const res = await fetch(url, { headers: NOMINATIM_HEADERS });
    if (res.ok) {
      const data: {
        lat: string;
        lon: string;
        display_name: string;
        address?: {
          road?: string;
          house_number?: string;
          city?: string;
          town?: string;
          village?: string;
          state?: string;
        };
      } = await res.json();
      const a = data.address || {};
      const streetLine = [a.road, a.house_number].filter(Boolean).join(", ");
      const city = a.city || a.town || a.village || "";
      const parts: string[] = [];
      if (streetLine) parts.push(streetLine);
      else if (city) parts.push(city);
      if (a.state) parts.push(`(${a.state})`);
      return {
        lat,
        lng,
        city,
        address: parts.join(" ").trim(),
        displayName: data.display_name,
      };
    }
  } catch {
    // fallback тоже упал
  }

  return null;
}

// searchAddress: свободный запрос → список результатов для dropdown.
// Photon первым, Nominatim — если Photon пуст.
export async function searchAddress(query: string): Promise<GeocodedPlace[]> {
  const q = query.trim();
  if (q.length < 3) return [];

  // Photon
  try {
    const url = `${PHOTON_BASE}/api?q=${encodeURIComponent(q)}&lang=ru&limit=5`;
    const res = await fetch(url);
    if (res.ok) {
      const data: PhotonResponse = await res.json();
      if (data.features && data.features.length > 0) {
        return data.features.map(photonFeatureToPlace).filter((p) => p.city || p.address);
      }
    }
  } catch {
    // fallback
  }

  // Nominatim fallback
  try {
    const url = `${NOMINATIM_BASE}/search?q=${encodeURIComponent(q)}&format=json&addressdetails=1&accept-language=ru&limit=5`;
    const res = await fetch(url, { headers: NOMINATIM_HEADERS });
    if (res.ok) {
      const data: Array<{
        lat: string;
        lon: string;
        display_name: string;
        address?: {
          road?: string;
          house_number?: string;
          city?: string;
          town?: string;
          village?: string;
          state?: string;
        };
      }> = await res.json();
      return data.map((d) => {
        const a = d.address || {};
        const streetLine = [a.road, a.house_number].filter(Boolean).join(", ");
        const city = a.city || a.town || a.village || "";
        const parts: string[] = [];
        if (streetLine) parts.push(streetLine);
        else if (city) parts.push(city);
        if (a.state) parts.push(`(${a.state})`);
        return {
          lat: parseFloat(d.lat),
          lng: parseFloat(d.lon),
          city,
          address: parts.join(" ").trim(),
          displayName: d.display_name,
        };
      });
    }
  } catch {
    // всё упало
  }

  return [];
}