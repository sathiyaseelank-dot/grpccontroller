"use client"

import { useEffect, useState, useCallback } from "react"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Network, RefreshCw } from "lucide-react"
import { Button } from "@/components/ui/button"

interface Tunneler {
  id: string
  status: "ONLINE" | "OFFLINE"
  connector_id: string
  last_seen: string
}

export function TunnelersTable() {
  const [tunnelers, setTunnelers] = useState<Tunneler[]>([])
  const [loading, setLoading] = useState(true)
  const [lastRefresh, setLastRefresh] = useState<Date>(new Date())
  const [error, setError] = useState<string | null>(null)

  const fetchTunnelers = useCallback(async () => {
    try {
      setError(null)
      const res = await fetch("/api/admin/tunnelers")
      if (!res.ok) {
        const message = await res.text()
        throw new Error(message || "Failed to load tunnelers")
      }
      const data: Tunneler[] = await res.json()
      setTunnelers(data)
    } catch (err) {
      setTunnelers([])
      setError(err instanceof Error ? err.message : "Failed to load tunnelers")
    } finally {
      setLoading(false)
      setLastRefresh(new Date())
    }
  }, [])

  useEffect(() => {
    fetchTunnelers()
    const interval = setInterval(fetchTunnelers, 10000)
    return () => clearInterval(interval)
  }, [fetchTunnelers])

  const onlineCount = tunnelers.filter((t) => t.status === "ONLINE").length
  const offlineCount = tunnelers.filter((t) => t.status === "OFFLINE").length

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">Tunnelers</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Monitor tunnelers and their current status.
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-3">
        <Card className="border-border bg-card">
          <CardContent className="flex items-center gap-3 p-4">
            <div className="flex h-9 w-9 items-center justify-center rounded-md bg-secondary">
              <Network className="h-4 w-4 text-muted-foreground" />
            </div>
            <div>
              <p className="text-2xl font-bold text-foreground">{tunnelers.length}</p>
              <p className="text-xs text-muted-foreground">Total</p>
            </div>
          </CardContent>
        </Card>
        <Card className="border-border bg-card">
          <CardContent className="flex items-center gap-3 p-4">
            <div className="flex h-9 w-9 items-center justify-center rounded-md bg-primary/10">
              <span className="h-2.5 w-2.5 rounded-full bg-primary" />
            </div>
            <div>
              <p className="text-2xl font-bold text-primary">{onlineCount}</p>
              <p className="text-xs text-muted-foreground">Online</p>
            </div>
          </CardContent>
        </Card>
        <Card className="border-border bg-card">
          <CardContent className="flex items-center gap-3 p-4">
            <div className="flex h-9 w-9 items-center justify-center rounded-md bg-destructive/10">
              <span className="h-2.5 w-2.5 rounded-full bg-destructive" />
            </div>
            <div>
              <p className="text-2xl font-bold text-destructive">{offlineCount}</p>
              <p className="text-xs text-muted-foreground">Offline</p>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card className="border-border bg-card">
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle className="text-base text-foreground">All Tunnelers</CardTitle>
            <CardDescription className="text-muted-foreground">
              Auto-refreshes every 10 seconds. Last updated:{" "}
              {lastRefresh.toLocaleTimeString()}
            </CardDescription>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={fetchTunnelers}
            className="border-border text-foreground hover:bg-secondary bg-transparent"
            aria-label="Refresh tunnelers"
          >
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>
        </CardHeader>
        <CardContent className="px-0 pb-0">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <RefreshCw className="h-5 w-5 animate-spin text-muted-foreground" />
            </div>
          ) : error ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16">
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-destructive/10">
                <Network className="h-6 w-6 text-destructive" />
              </div>
              <div className="text-center">
                <p className="text-sm font-medium text-foreground">Failed to load tunnelers</p>
                <p className="mt-1 text-sm text-muted-foreground">{error}</p>
              </div>
            </div>
          ) : tunnelers.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16">
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-secondary">
                <Network className="h-6 w-6 text-muted-foreground" />
              </div>
              <div className="text-center">
                <p className="text-sm font-medium text-foreground">No tunnelers found</p>
                <p className="mt-1 text-sm text-muted-foreground">
                  Create a tunneler token and run the install command on a server.
                </p>
              </div>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow className="border-border hover:bg-transparent">
                  <TableHead className="text-muted-foreground">Tunneler ID</TableHead>
                  <TableHead className="text-muted-foreground">Status</TableHead>
                  <TableHead className="text-muted-foreground">Connector ID</TableHead>
                  <TableHead className="text-muted-foreground">Last Seen</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tunnelers.map((tunneler) => (
                  <TableRow key={tunneler.id} className="border-border">
                    <TableCell className="font-mono text-sm text-foreground">
                      {tunneler.id}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant="outline"
                        className={
                          tunneler.status === "ONLINE"
                            ? "border-primary/30 bg-primary/10 text-primary"
                            : "border-destructive/30 bg-destructive/10 text-destructive"
                        }
                      >
                        <span
                          className={`mr-1.5 inline-block h-1.5 w-1.5 rounded-full ${
                            tunneler.status === "ONLINE" ? "bg-primary" : "bg-destructive"
                          }`}
                        />
                        {tunneler.status === "ONLINE" ? "Online" : "Offline"}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-sm text-muted-foreground">
                      {tunneler.connector_id}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {tunneler.last_seen}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
