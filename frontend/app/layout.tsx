import "./globals.css";

export const metadata = {
  title: "RTealytics",
  description: "Real-time analytics dashboard",
};

import { PropsWithChildren } from 'react';

export default function RootLayout({ children }: PropsWithChildren) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
