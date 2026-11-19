// Кастомный PNG-маркер для Leaflet.
// Файл: web/public/map-pin.png (80×104 retina, 40×52 в CSS).
// anchor — острие пина внизу по центру: [width/2, height].

import type { Icon, icon as IconFactory } from "leaflet";

const ICON_URL = "/map-pin.png";
const ICON_WIDTH = 22;
const ICON_HEIGHT = 30;

let cachedIcon: Icon | null = null;

export function getMarkerIcon(L: { icon: typeof IconFactory }): Icon {
  if (cachedIcon) return cachedIcon;
  cachedIcon = L.icon({
    iconUrl: ICON_URL,
    iconSize: [ICON_WIDTH, ICON_HEIGHT],
    iconAnchor: [ICON_WIDTH / 2, ICON_HEIGHT], // острие внизу
    popupAnchor: [0, -ICON_HEIGHT], // если появятся попапы
    className: "namylimsya-marker-pin",
  });
  return cachedIcon;
}

export const MARKER_ICON_SIZE: [number, number] = [ICON_WIDTH, ICON_HEIGHT];
export const MARKER_ICON_ANCHOR: [number, number] = [
  ICON_WIDTH / 2,
  ICON_HEIGHT,
];