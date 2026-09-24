import "./globals.css";
import AppShell from "@/components/AppShell";

export const metadata = {
  title: "MyanKafe Finance",
  description: "Multi-entity double-entry finance platform",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body><AppShell>{children}</AppShell></body>
    </html>
  );
}
