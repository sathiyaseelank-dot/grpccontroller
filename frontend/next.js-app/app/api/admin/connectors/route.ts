import { NextResponse } from "next/server"

export async function GET() {
  const baseUrl = process.env.ADMIN_API_URL
  const authToken = process.env.ADMIN_AUTH_TOKEN

  if (!baseUrl || !authToken) {
    return NextResponse.json(
      { error: "ADMIN_API_URL or ADMIN_AUTH_TOKEN not configured" },
      { status: 500 }
    )
  }

  try {
    const res = await fetch(`${baseUrl}/api/admin/connectors`, {
      headers: {
        Authorization: `Bearer ${authToken}`,
      },
      cache: "no-store",
    })

    const text = await res.text()
    if (!res.ok) {
      return NextResponse.json(
        { error: text || "Failed to fetch connectors" },
        { status: res.status }
      )
    }

    const data = text ? JSON.parse(text) : []
    return NextResponse.json(data, { status: 200 })
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "Upstream error" },
      { status: 502 }
    )
  }
}
