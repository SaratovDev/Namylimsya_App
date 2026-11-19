"use client";

import { useEffect, useRef, useState } from "react";
import dynamic from "next/dynamic";
import { Loader2, MapPin, Search } from "lucide-react";
import { useMapEvents } from "react-leaflet";
import type { Map as LeafletMap } from "leaflet";
import { Input } from "@/components/ui/input";
import { reverseGeocode, searchAddress, GeocodedPlace } from "@/lib/geocoding";
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

type LeafletNS = typeof import("leaflet");

const SARATOV = { lat: 51.5336, lng: 46.0343 };

type Props = {
  /** Текущий город (для fallback при неудачном reverse-geocode). */
  city: string;
  /** Текущий адрес (для fallback при неудачном reverse-geocode). */
  address: string;
  lat: number;
  lng: number;
  onChange: (data: {
    city: string;
    address: string;
    lat: number;
    lng: number;
  }) => void;
};

function MapClickHandler({
  onClick,
}: {
  onClick: (lat: number, lng: number) => void;
}) {
  useMapEvents({
    click(e) {
      onClick(e.latlng.lat, e.latlng.lng);
    },
  });
  return null;
}

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

export function MapPicker({ city, address, lat, lng, onChange }: Props) {
  const L = useLeaflet();
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [results, setResults] = useState<GeocodedPlace[]>([]);
  const [searching, setSearching] = useState(false);
  const [showResults, setShowResults] = useState(false);

  const mapRef = useRef<LeafletMap | null>(null);

  useEffect(() => {
    const q = searchQuery.trim();
    if (q.length < 3) return;
    let cancelled = false;
    const t = setTimeout(async () => {
      const r = await searchAddress(q);
      if (cancelled) return;
      setResults(r);
      setSearching(false);
      setShowResults(true);
    }, 400);
    return () => {
      cancelled = true;
      clearTimeout(t);
    };
  }, [searchQuery]);

  const handleSearchChange = (value: string) => {
    setSearchQuery(value);
    const q = value.trim();
    // searching выставляется синхронно в обработчике ввода (не в effect).
    setSearching(q.length >= 3);
    if (q.length < 3) {
      setResults([]);
      setShowResults(false);
    }
  };

  const handleMapClick = async (newLat: number, newLng: number) => {
    setLoading(true);
    try {
      const place = await reverseGeocode(newLat, newLng);
      if (place) {
        onChange({
          city: place.city,
          address: place.address,
          lat: newLat,
          lng: newLng,
        });
      } else {
        // Reverse не сработал — сохраняем координаты и текущие city/address
        // (пользователь мог их отредактировать вручную).
        onChange({ city, address, lat: newLat, lng: newLng });
      }
    } finally {
      setLoading(false);
    }
  };

  const handleSelectResult = (r: GeocodedPlace) => {
    onChange({ city: r.city, address: r.address, lat: r.lat, lng: r.lng });
    setSearchQuery("");
    setResults([]);
    setShowResults(false);
    mapRef.current?.flyTo([r.lat, r.lng], 16, { duration: 0.5 });
  };

  const centerLat = lat || SARATOV.lat;
  const centerLng = lng || SARATOV.lng;
  const hasPoint = lat !== 0 && lng !== 0;

  return (
    <div className="space-y-3">
      {/* Поиск адреса */}
      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
        <Input
          value={searchQuery}
          onChange={(e) => handleSearchChange(e.target.value)}
          onFocus={() => setShowResults(results.length > 0)}
          onBlur={() => setTimeout(() => setShowResults(false), 200)}
          placeholder="Найти адрес: улица, город…"
          className="pl-9"
        />
        {searching && (
          <Loader2 className="absolute right-3 top-1/2 -translate-y-1/2 h-4 w-4 animate-spin text-muted-foreground" />
        )}
        {showResults && results.length > 0 && (
          <div className="absolute top-full left-0 right-0 mt-1 bg-popover border rounded-lg shadow-lg z-2000 max-h-64 overflow-y-auto">
            {results.map((r, i) => (
              <button
                key={`${r.lat}-${r.lng}-${i}`}
                type="button"
                onClick={() => handleSelectResult(r)}
                className="w-full text-left px-3 py-2 text-sm hover:bg-muted transition-colors border-b last:border-0"
              >
                <div className="font-medium truncate">{r.displayName}</div>
                {(r.city || r.address) && (
                  <div className="text-xs text-muted-foreground truncate">
                    {r.address || r.city}
                  </div>
                )}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Карта */}
      <div className="relative rounded-lg overflow-hidden border h-65">
        <MapContainer
          center={[centerLat, centerLng]}
          zoom={hasPoint ? 16 : 13}
          scrollWheelZoom={true}
          style={{ width: "100%", height: "100%" }}
          ref={mapRef}
        >
          <TileLayer
            url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
            attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          />
          {hasPoint && L && (
            <Marker position={[lat, lng]} icon={getMarkerIcon(L)} />
          )}
          <MapClickHandler onClick={handleMapClick} />
        </MapContainer>

        {loading && (
          <div className="absolute top-2 right-2 bg-background/90 backdrop-blur px-2 py-1 rounded text-xs flex items-center gap-1 z-1000">
            <Loader2 className="h-3 w-3 animate-spin" />
            Определяем адрес…
          </div>
        )}

        {!hasPoint && !loading && (
          <div className="absolute bottom-2 left-2 right-2 bg-background/90 backdrop-blur px-3 py-2 rounded text-xs text-center z-1000 pointer-events-none">
            <MapPin className="h-3 w-3 inline mr-1" />
            Кликните на карте, чтобы указать место
          </div>
        )}
      </div>

      {hasPoint && (
        <p className="text-xs text-muted-foreground">
          Координаты: {lat.toFixed(4)}, {lng.toFixed(4)}
        </p>
      )}
    </div>
  );
}