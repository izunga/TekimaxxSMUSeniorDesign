import { NextResponse } from "next/server";

const ALLOWED_ORIGINS = [
  "http://localhost:8080",
  "http://localhost:8001",
  "http://localhost:8002",
  "http://localhost:3001",
];

type ProxyBody = {
  url: string;
  method?: string;
  headers?: Record<string, string>;
  body?: unknown;
};

export async function POST(request: Request) {
  const payload = (await request.json()) as ProxyBody;
  const target = new URL(payload.url);

  if (!ALLOWED_ORIGINS.includes(target.origin)) {
    return NextResponse.json(
      { ok: false, status: 400, error: `Blocked non-demo target: ${target.origin}` },
      { status: 400 }
    );
  }

  const method = payload.method ?? "GET";
  const hasBody = payload.body !== undefined && method !== "GET" && method !== "HEAD";
  const upstream = await fetch(target.toString(), {
    method,
    headers: {
      ...(hasBody ? { "Content-Type": "application/json" } : {}),
      ...(payload.headers ?? {}),
    },
    body: hasBody
      ? typeof payload.body === "string"
        ? payload.body
        : JSON.stringify(payload.body)
      : undefined,
    cache: "no-store",
  });

  const contentType = upstream.headers.get("content-type") ?? "";
  const data = contentType.includes("application/json") ? await upstream.json() : await upstream.text();

  return NextResponse.json({
    ok: upstream.ok,
    status: upstream.status,
    statusText: upstream.statusText,
    data,
  });
}
