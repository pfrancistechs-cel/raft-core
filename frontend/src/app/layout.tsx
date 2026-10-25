import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "RaftCore | Distributed Consensus & Quorum State Machine",
  description: "Interactive distributed Raft consensus cluster simulator with leader election, log replication, and network partition tolerance.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-[#070914] text-slate-100 antialiased selection:bg-cyan-500 selection:text-black">
        {children}
      </body>
    </html>
  );
}

