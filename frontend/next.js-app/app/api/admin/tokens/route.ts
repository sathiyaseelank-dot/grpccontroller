import { NextResponse } from "next/server"

export const runtime = "nodejs"
export const dynamic = "force-dynamic"

export async function POST() {
  const baseUrl = process.env.ADMIN_API_URL
  const authToken = process.env.ADMIN_AUTH_TOKEN

  if (!baseUrl || !authToken) {
    return NextResponse.json(
      { error: "ADMIN_API_URL or ADMIN_AUTH_TOKEN not configured" },
      { status: 500 }
    )
  }

  try {
    const res = await fetch(`${baseUrl}/api/admin/tokens`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${authToken}`,
      },
      cache: "no-store",
    })

    const body = await res.text()
    return new Response(body, {
      status: res.status,
      headers: {
        "Content-Type": res.headers.get("content-type") || "application/json",
      },
    })
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "Upstream error" },
      { status: 502 }
    )
  }
}
