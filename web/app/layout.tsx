import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Намылимся",
  description: "Площадка для организации живых встреч",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="ru">
      <body className="antialiased">{children}</body>
    </html>
  );
}