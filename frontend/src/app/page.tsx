"use client"

import { useRouter } from "next/navigation"
import Link from "next/link"
import { ArrowRight } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { ModeToggle } from "@/components/mode-toggle"
import { Logo } from "@/components/logo"

export default function Home() {
  const router = useRouter()

  return (
    <div className="min-h-screen flex flex-col">
      <header className="border-b px-6 h-12 flex items-center justify-between shrink-0">
        <Link href="/" className="flex items-center gap-3">
          <Logo className="h-5 w-auto" />
        </Link>
        <div className="flex items-center gap-2">
          <ModeToggle />
          <Link href="/login">
            <Button variant="ghost" size="sm">Log in</Button>
          </Link>
          <Link href="/signup">
            <Button size="sm">Sign up</Button>
          </Link>
        </div>
      </header>

      <main className="flex flex-col items-center px-4 pt-24 pb-20 text-center flex-1 animate-fade-in">
        <h1 className="text-5xl font-bold tracking-tight leading-tight max-w-lg">
          Find exploits<br />
          <span className="text-orange">before they find you</span>
        </h1>
        <p className="text-muted-foreground mt-4 max-w-md text-sm">
          AI-powered security analysis in an isolated sandbox. Instant results. Auto-wiped after every scan.
        </p>
        <Link href="/signup">
          <Button size="lg" className="mt-8 gap-2">
            Start Scanning Free
            <ArrowRight className="h-4 w-4" />
          </Button>
        </Link>

        <div className="grid md:grid-cols-2 gap-4 mt-16 max-w-xl w-full">
          <Card className="text-left animate-slide-up" style={{ animationDelay: "100ms", animationFillMode: "backwards" }}>
            <CardHeader>
              <CardTitle>Exploit Scanner</CardTitle>
              <CardDescription>
                Upload a codebase or GitHub repo. Get CVE mapping, attack PoCs, and remediation steps.
              </CardDescription>
            </CardHeader>
          </Card>
          <Card className="text-left animate-slide-up" style={{ animationDelay: "160ms", animationFillMode: "backwards" }}>
            <CardHeader>
              <CardTitle>Malware Scanner</CardTitle>
              <CardDescription>
                Analyze binaries and dependencies. Get behavioral timelines and registry forensics.
              </CardDescription>
            </CardHeader>
          </Card>
        </div>
      </main>

      <footer className="text-center py-6 text-xs text-muted-foreground">
        Lysis · AI Code Security
      </footer>
    </div>
  )
}
