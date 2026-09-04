import type { Metadata } from "next"
import "@/styles/globals.css"
import { RootLayoutClient } from "./layout-client"

export const metadata: Metadata = {
  title: "Lysis — AI Code Security Scanner",
  description: "AI-powered security analysis for codebases. Find exploits and malware with automated sandbox analysis.",
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <meta httpEquiv="Content-Security-Policy" content="default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; base-uri 'self'; form-action 'self';" />
        <link rel="icon" href="/favicon.ico" sizes="256x256" type="image/x-icon" />
      </head>
      <body className="font-sans antialiased">
        <RootLayoutClient>{children}</RootLayoutClient>
      </body>
    </html>
  )
}
