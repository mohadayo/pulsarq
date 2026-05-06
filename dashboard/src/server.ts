import express, { Request, Response, NextFunction } from "express";

const app = express();

const API_GATEWAY_URL = process.env.API_GATEWAY_URL || "http://localhost:5000";
const WORKER_URL = process.env.WORKER_URL || "http://localhost:5001";
const DASHBOARD_PORT = parseInt(process.env.DASHBOARD_PORT || "3000", 10);
const LOG_LEVEL = process.env.LOG_LEVEL || "INFO";

function log(level: string, message: string): void {
  if (level === "DEBUG" && LOG_LEVEL !== "DEBUG") return;
  const ts = new Date().toISOString();
  console.log(`${ts} [${level}] dashboard: ${message}`);
}

interface ServiceStatus {
  name: string;
  url: string;
  status: "up" | "down";
  details?: Record<string, unknown>;
}

async function checkService(
  name: string,
  url: string
): Promise<ServiceStatus> {
  try {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 3000);
    const resp = await fetch(`${url}/health`, { signal: controller.signal });
    clearTimeout(timeout);
    const data = await resp.json();
    return { name, url, status: "up", details: data as Record<string, unknown> };
  } catch {
    return { name, url, status: "down" };
  }
}

app.get("/health", (_req: Request, res: Response) => {
  res.json({
    service: "dashboard",
    status: "healthy",
    timestamp: Date.now() / 1000,
  });
});

app.get("/api/status", async (_req: Request, res: Response) => {
  log("INFO", "Fetching service statuses");
  const services = await Promise.all([
    checkService("api-gateway", API_GATEWAY_URL),
    checkService("task-worker", WORKER_URL),
  ]);
  res.json({ services, checked_at: new Date().toISOString() });
});

app.get("/api/tasks", async (_req: Request, res: Response) => {
  try {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 5000);
    const resp = await fetch(`${API_GATEWAY_URL}/api/tasks`, {
      signal: controller.signal,
    });
    clearTimeout(timeout);
    const data = await resp.json();
    res.json(data);
  } catch (err) {
    log("ERROR", `Failed to fetch tasks: ${err}`);
    res.status(503).json({ error: "API gateway unavailable" });
  }
});

app.get("/", (_req: Request, res: Response) => {
  res.send(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>PulsarQ Dashboard</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: system-ui, sans-serif; background: #0f172a; color: #e2e8f0; padding: 2rem; }
    h1 { font-size: 2rem; margin-bottom: 1.5rem; color: #38bdf8; }
    .card { background: #1e293b; border-radius: 8px; padding: 1.5rem; margin-bottom: 1rem; }
    .status { display: inline-block; width: 12px; height: 12px; border-radius: 50%; margin-right: 8px; }
    .up { background: #22c55e; }
    .down { background: #ef4444; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 1rem; }
    #tasks { margin-top: 2rem; }
    table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
    th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid #334155; }
    th { color: #94a3b8; font-weight: 600; }
  </style>
</head>
<body>
  <h1>PulsarQ Dashboard</h1>
  <div class="grid" id="services"></div>
  <div id="tasks">
    <h2 style="margin-bottom: 1rem;">Tasks</h2>
    <div class="card"><table><thead><tr><th>ID</th><th>Name</th><th>Status</th><th>Priority</th></tr></thead><tbody id="task-list"></tbody></table></div>
  </div>
  <script>
    async function refresh() {
      try {
        const sr = await fetch('/api/status');
        const sd = await sr.json();
        document.getElementById('services').innerHTML = sd.services.map(s =>
          '<div class="card"><span class="status ' + s.status + '"></span><strong>' + s.name + '</strong><br><small>' + s.url + '</small></div>'
        ).join('');
      } catch(e) { console.error(e); }
      try {
        const tr = await fetch('/api/tasks');
        const td = await tr.json();
        document.getElementById('task-list').innerHTML = (Array.isArray(td) ? td : []).map(t =>
          '<tr><td>' + (t.id||'').substring(0,8) + '</td><td>' + (t.name||'') + '</td><td>' + (t.status||'') + '</td><td>' + (t.priority||'') + '</td></tr>'
        ).join('');
      } catch(e) { console.error(e); }
    }
    refresh();
    setInterval(refresh, 5000);
  </script>
</body>
</html>`);
});

app.use((err: Error, _req: Request, res: Response, _next: NextFunction) => {
  log("ERROR", `Unhandled error: ${err.message}`);
  res.status(500).json({ error: "Internal server error" });
});

if (require.main === module) {
  app.listen(DASHBOARD_PORT, () => {
    log("INFO", `Dashboard listening on port ${DASHBOARD_PORT}`);
  });
}

export { app, checkService };
