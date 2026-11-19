"use client";

import { useEffect, useState } from "react";
import dynamic from "next/dynamic";
import { getMarkerIcon } from "@/lib/map-marker";

const MapContainer = dynamic(
  () => import("react-leaflet").then((m) => m.MapContainer),
  { ssr: false }
);
const TileLayer = dynamic(
  () => import("react-leaflet").then((m) => m.TileLayer),
  { ssr: false }
);
const Marker = dynamic(
  () => import("react-leaflet").then((m) => m.Marker),
  { ssr: false }
);
const Circle = dynamic(
  () => import("react-leaflet").then((m) => m.Circle),
  { ssr: false }
);

type LeafletNS = typeof import("leaflet");

type Props = {
  lat: number;
  lng: number;
  hideExactLocation: boolean;
  canSeeExact: boolean;
  city: string;
  /**
   * Tailwind-класс высоты. По умолчанию h-60 (240px) — для карточек
   * на ленте (раскрытая карточка). На EventDetail можно передать h-80
   * (320px) или h-full (заполнить родительский контейнер).
   */
  heightClass?: string;
};

function useLeaflet(): LeafletNS | null {
  const [L, setL] = useState<LeafletNS | null>(null);
  useEffect(() => {
    let cancelled = false;
    void import("leaflet").then((mod) => {
      if (!cancelled) setL(mod);
    });
    return () => {
      cancelled = true;
    };
  }, []);
  return L;
}

export function EventMap({
  lat,
  lng,
  hideExactLocation,
  canSeeExact,
  city,
  heightClass = "h-60",
}: Props) {
  const L = useLeaflet();
  const displayLat = lat;
  const displayLng = lng;
  const isMasked = hideExactLocation && !canSeeExact;

  return (
    <div
      className={`relative w-full ${heightClass} rounded-lg overflow-hidden border border-border`}
      aria-label={`Карта события в городе ${city}`}
    >
      <MapContainer
        center={[displayLat, displayLng]}
        zoom={isMasked ? 12 : 16}
        scrollWheelZoom={false}
        style={{ width: "100%", height: "100%" }}
        attributionControl={true}
      >
        <TileLayer
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
        />

        {isMasked ? (
          <Circle
            center={[displayLat, displayLng]}
            radius={1000}
            pathOptions={{
              color: "#b52e00",
              fillColor: "#b52e00",
              fillOpacity: 0.15,
              weight: 2,
            }}
          />
        ) : (
          L && (
            <Marker
              position={[displayLat, displayLng]}
              icon={getMarkerIcon(L)}
            />
          )
        )}
      </MapContainer>

      {isMasked && (
        <div className="absolute bottom-2 left-2 right-2 bg-background/90 backdrop-blur px-3 py-2 rounded text-xs pointer-events-none z-1000">
          Точный адрес — после одобрения заявки
        </div>
      )}
    </div>
  );
}