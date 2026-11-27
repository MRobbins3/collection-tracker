// Forwards 127.0.0.1:8080 to the API container, for running Playwright
// inside the web container (`docker compose exec web bun run test:e2e`).
// The browser calls the API at NUXT_PUBLIC_API_BASE (http://localhost:8080),
// which inside the web container is not the API, so signed-in flows would
// fail. playwright.config.ts starts this only when E2E_API_PROXY_TARGET is
// set (docker-compose.yml sets it); CI and host runs reach the API directly.
import net from "node:net";

const target = new URL(process.env.E2E_API_PROXY_TARGET ?? "http://api:8080");
const port = Number(target.port || 80);

net
  .createServer((client) => {
    const upstream = net.connect(port, target.hostname);
    client.pipe(upstream);
    upstream.pipe(client);
    client.on("error", () => upstream.destroy());
    upstream.on("error", () => client.destroy());
  })
  .listen(8080, "127.0.0.1", () => {
    console.log(`api-proxy: 127.0.0.1:8080 -> ${target.hostname}:${port}`);
  });
