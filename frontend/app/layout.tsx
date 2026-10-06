import "./globals.css";
import AppShell from "@/components/AppShell";

export const metadata = {
  title: "MyanKafe Finance",
  description: "Multi-entity double-entry finance platform",
  icons: {
    icon: [{ url: "/brand/logo-head.svg", type: "image/svg+xml" }],
  },
};

export const viewport = {
  themeColor: "#240C03",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body><AppShell>{children}</AppShell></body>
    </html>
  );
}
