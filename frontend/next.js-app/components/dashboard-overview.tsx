"use client"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Network, ShieldCheck, ShieldAlert } from "lucide-react"

export function DashboardOverview() {
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
            <div className="text-3xl font-bold text-foreground">--</div>
            <p className="mt-1 text-xs text-muted-foreground">Across all networks</p>
          </CardContent>
        </Card>

        <Card className="border-border bg-card">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Online
            </CardTitle>
            <ShieldCheck className="h-4 w-4 text-primary" />
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold text-primary">--</div>
            <p className="mt-1 text-xs text-muted-foreground">Active connections</p>
          </CardContent>
        </Card>

        <Card className="border-border bg-card">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Offline
            </CardTitle>
            <ShieldAlert className="h-4 w-4 text-destructive" />
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold text-destructive">--</div>
            <p className="mt-1 text-xs text-muted-foreground">Require attention</p>
          </CardContent>
        </Card>
      </div>

      <Card className="border-border bg-card">
        <CardHeader>
          <CardTitle className="text-base text-foreground">Getting Started</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3 text-sm text-muted-foreground">
          <p>
            <span className="font-medium text-foreground">1.</span> Generate a connector token from the{" "}
            <span className="font-medium text-primary">Create Connector Token</span> page.
          </p>
          <p>
            <span className="font-medium text-foreground">2.</span> Run the install command on your target server.
          </p>
          <p>
            <span className="font-medium text-foreground">3.</span> Monitor your connectors from the{" "}
            <span className="font-medium text-primary">Connectors</span> page.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
