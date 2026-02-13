"use client"

import { useCallback, useEffect, useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Activity, Network, ShieldCheck, ShieldAlert } from "lucide-react"

interface Connector {
  id: string
  status: "ONLINE" | "OFFLINE"
  private_ip: string
  last_seen: string
  version?: string
}

interface Tunneler {
  id: string
  status: "ONLINE" | "OFFLINE"
  connector_id: string
  last_seen: string
}

export function DashboardOverview() {
  const [connectors, setConnectors] = useState<Connector[]>([])
  const [tunnelers, setTunnelers] = useState<Tunneler[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [lastRefresh, setLastRefresh] = useState<Date>(new Date())

  const fetchStats = useCallback(async () => {
    try {
      setError(null)
      const [connectorsRes, tunnelersRes] = await Promise.all([
        fetch("/api/admin/connectors"),
        fetch("/api/admin/tunnelers"),
      ])

      if (!connectorsRes.ok) {
        const message = await connectorsRes.text()
        throw new Error(message || "Failed to load connectors")
      }
      if (!tunnelersRes.ok) {
        const message = await tunnelersRes.text()
        throw new Error(message || "Failed to load tunnelers")
      }

      const connectorsData: Connector[] = await connectorsRes.json()
      const tunnelersData: Tunneler[] = await tunnelersRes.json()
      setConnectors(connectorsData)
      setTunnelers(tunnelersData)
    } catch (err) {
      setConnectors([])
      setTunnelers([])
      setError(err instanceof Error ? err.message : "Failed to load dashboard data")
    } finally {
      setLoading(false)
      setLastRefresh(new Date())
    }
  }, [])

  useEffect(() => {
    fetchStats()
    const interval = setInterval(fetchStats, 10000)
    return () => clearInterval(interval)
  }, [fetchStats])

  const connectorsOnline = connectors.filter((c) => c.status === "ONLINE").length
  const connectorsOffline = connectors.filter((c) => c.status === "OFFLINE").length
  const tunnelersOnline = tunnelers.filter((t) => t.status === "ONLINE").length
  const tunnelersOffline = tunnelers.filter((t) => t.status === "OFFLINE").length

  const latestConnector = connectors[0]
  const latestTunneler = tunnelers[0]

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">Dashboard</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Overview of your zero-trust connector network.
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Card className="border-border bg-card">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Total Connectors
            </CardTitle>
            <Network className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold text-foreground">
              {loading ? "--" : connectors.length}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">Across all networks</p>
          </CardContent>
        </Card>

        <Card className="border-border bg-card">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Online Connectors
            </CardTitle>
            <ShieldCheck className="h-4 w-4 text-primary" />
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold text-primary">
              {loading ? "--" : connectorsOnline}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">Active connections</p>
          </CardContent>
        </Card>

        <Card className="border-border bg-card">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Offline Connectors
            </CardTitle>
            <ShieldAlert className="h-4 w-4 text-destructive" />
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold text-destructive">
              {loading ? "--" : connectorsOffline}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">Require attention</p>
          </CardContent>
        </Card>

        <Card className="border-border bg-card">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Total Tunnelers
            </CardTitle>
            <Network className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold text-foreground">
              {loading ? "--" : tunnelers.length}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">Across all connectors</p>
          </CardContent>
        </Card>

        <Card className="border-border bg-card">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Online Tunnelers
            </CardTitle>
            <ShieldCheck className="h-4 w-4 text-primary" />
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold text-primary">
              {loading ? "--" : tunnelersOnline}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">Active tunnelers</p>
          </CardContent>
        </Card>

        <Card className="border-border bg-card">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Offline Tunnelers
            </CardTitle>
            <ShieldAlert className="h-4 w-4 text-destructive" />
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold text-destructive">
              {loading ? "--" : tunnelersOffline}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">Require attention</p>
          </CardContent>
        </Card>
      </div>

      <Card className="border-border bg-card">
        <CardHeader>
          <CardTitle className="text-base text-foreground">Latest Activity</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3 text-sm text-muted-foreground">
          {error ? (
            <p className="text-destructive">{error}</p>
          ) : (
            <>
              <div className="flex items-start gap-3">
                <Activity className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-foreground">Connector Activity</p>
                  <p className="text-muted-foreground">
                    {latestConnector
                      ? `${latestConnector.id} • ${latestConnector.last_seen}`
                      : "No connector activity yet"}
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <Activity className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-foreground">Tunneler Activity</p>
                  <p className="text-muted-foreground">
                    {latestTunneler
                      ? `${latestTunneler.id} • ${latestTunneler.last_seen}`
                      : "No tunneler activity yet"}
                  </p>
                </div>
              </div>
              <p className="text-xs text-muted-foreground">
                Last updated: {lastRefresh.toLocaleTimeString()}
              </p>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
