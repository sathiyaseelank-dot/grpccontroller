"use client"

import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { AlertTriangle, Check, Copy, KeyRound, Loader2 } from "lucide-react"

interface TokenResponse {
  token: string
  expires_at: string
}

export function CreateToken() {
  const [token, setToken] = useState<TokenResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleCreateToken = async () => {
    setLoading(true)
    setToken(null)
    setCopied(false)
    setError(null)

    try {
      const res = await fetch("/api/admin/tokens", { method: "POST" })
      if (!res.ok) {
        const message = await res.text()
        throw new Error(message || "Failed to create token")
      }
      const data: TokenResponse = await res.json()
      setToken(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create token")
    } finally {
      setLoading(false)
    }
  }

  const installCommand = token
    ? `curl -fsSL https://raw.githubusercontent.com/sathiyaseelank-dot/grpccontroller/main/scripts/setup.sh | sudo CONNECTOR_ID="connector-local-01" CONTROLLER_ADDR="127.0.0.1:8443" MY_CONNECTOR_TOKEN="${token.token}" bash`
    : ""

  const handleCopy = async () => {
    await navigator.clipboard.writeText(installCommand)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">
          Create Connector Token
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Generate a one-time token to register a new connector to your network.
        </p>
      </div>

      <Card className="border-border bg-card">
        <CardHeader>
          <CardTitle className="text-base text-foreground">Generate Token</CardTitle>
          <CardDescription className="text-muted-foreground">
            Each token can only be used once and expires after 24 hours.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button onClick={handleCreateToken} disabled={loading}>
            {loading ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" />
                Generating...
              </>
            ) : (
              <>
                <KeyRound className="h-4 w-4" />
                Create Connector Token
              </>
            )}
          </Button>
          {error && (
            <p className="mt-3 text-sm text-destructive" role="alert">
              {error}
            </p>
          )}
        </CardContent>
      </Card>

      {token && (
        <Card className="border-border bg-card">
          <CardHeader className="pb-3">
            <div className="flex items-center gap-2">
              <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary/10">
                <Check className="h-4 w-4 text-primary" />
              </div>
              <CardTitle className="text-base text-foreground">Token Created</CardTitle>
            </div>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            {/* Warning */}
            <div className="flex items-start gap-3 rounded-md border border-destructive/30 bg-destructive/5 p-3">
              <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
              <p className="text-sm font-medium text-destructive">
                This token will not be shown again. Copy it now.
              </p>
            </div>

            {/* Token display */}
            <div className="flex flex-col gap-1">
              <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                Token
              </span>
              <code className="break-all rounded-md border border-border bg-secondary px-3 py-2 font-mono text-sm text-foreground">
                {token.token}
              </code>
            </div>

            {/* Expiry */}
            <div className="flex flex-col gap-1">
              <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                Expires At
              </span>
              <span className="text-sm text-foreground">
                {new Date(token.expires_at).toLocaleString()}
              </span>
            </div>

            {/* Install command */}
            <div className="flex flex-col gap-2">
              <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                Install Command
              </span>
              <div className="relative">
                <pre className="overflow-x-auto rounded-md border border-border bg-secondary p-4 font-mono text-sm leading-relaxed text-foreground">
                  {installCommand}
                </pre>
                <Button
                  size="sm"
                  variant="secondary"
                  className="absolute right-2 top-2 border border-border bg-card text-foreground hover:bg-secondary"
                  onClick={handleCopy}
                  aria-label="Copy install command"
                >
                  {copied ? (
                    <>
                      <Check className="h-3 w-3" />
                      Copied
                    </>
                  ) : (
                    <>
                      <Copy className="h-3 w-3" />
                      Copy
                    </>
                  )}
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
