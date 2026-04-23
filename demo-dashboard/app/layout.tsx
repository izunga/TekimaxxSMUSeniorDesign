import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Tekimax Platform Demo Dashboard",
  description: "Interactive test harness for backend verification",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
