"use client"

import { useState } from "react"
import { LoginForm } from "@/components/login-form"
import { AdminSidebar } from "@/components/admin-sidebar"
import { DashboardOverview } from "@/components/dashboard-overview"
import { CreateToken } from "@/components/create-token"
import { ConnectorsTable } from "@/components/connectors-table"

type Page = "dashboard" | "tokens" | "connectors"

export default function App() {
  const [loggedIn, setLoggedIn] = useState(false)
  const [activePage, setActivePage] = useState<Page>("dashboard")

  if (!loggedIn) {
    return <LoginForm onLogin={() => setLoggedIn(true)} />
  }

  return (
    <div className="flex min-h-screen flex-col md:flex-row">
      <AdminSidebar
        activePage={activePage}
        onNavigate={setActivePage}
        onLogout={() => {
          setLoggedIn(false)
          setActivePage("dashboard")
        }}
      />
      <main className="flex-1 overflow-auto">
        <div className="mx-auto max-w-5xl px-4 py-6 md:px-8 md:py-8">
          {activePage === "dashboard" && <DashboardOverview />}
          {activePage === "tokens" && <CreateToken />}
          {activePage === "connectors" && <ConnectorsTable />}
        </div>
      </main>
    </div>
  )
}
