import "./globals.css";
import AppShell from "@/components/AppShell";

export const metadata = {
  title: "MyanKafe Finance",
  description: "Multi-entity double-entry finance platform",
  icons: {
    icon: [{ url: "/brand/chieftain-logo.png", type: "image/png" }],
  },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body><AppShell>{children}</AppShell></body>
    </html>
  );
}
