import { component$ } from "@builder.io/qwik";
import type { DocumentHead } from "@builder.io/qwik-city";

interface Cred {
  label: string;
  value: string;
}

interface Route {
  path: string;
  note: string;
}

interface Service {
  name: string;
  url?: string;
  tunnelUrl?: string; // URL pública (cloudflared) si este servicio está expuesto ahora mismo
  login: boolean;
  desc: string;
  creds?: Cred[];
  routes?: Route[];
}

interface Group {
  title: string;
  services: Service[];
}

const groups: Group[] = [
  {
    title: "App interna",
    services: [
      {
        name: "todamark-web (ticketing)",
        url: "http://localhost:8080",
        tunnelUrl: "https://backup-bus-furthermore-align.trycloudflare.com",
        login: true,
        desc: "App React de gestión de tickets para agentes/soporte. Requiere login.",
        creds: [
          { label: "admin", value: "user" },
          { label: "agente", value: "agente123" },
          { label: "gerente", value: "gerente123" },
          { label: "viewer", value: "viewer123" },
        ],
      },
    ],
  },
  {
    title: "Portal público",
    services: [
      {
        name: "astro-frontend",
        url: "http://localhost:8082",
        tunnelUrl: "https://lung-progress-handbook-crest.trycloudflare.com",
        login: false,
        desc: "Páginas públicas: Kanban + reportes, landing de marketing, y portal de cliente.",
        routes: [
          { path: "/", note: "Kanban + reportes, sin login" },
          { path: "/landing", note: "sitio de marketing" },
          {
            path: "/mis-tickets",
            note: "portal de cliente — pedir email, no login",
          },
        ],
      },
    ],
  },
  {
    title: "WhatsApp",
    services: [
      {
        name: "evolution (Evolution API)",
        url: "http://localhost:3100",
        login: false,
        desc: "Motor de WhatsApp. La raíz sirve su propia UI de manager (QR, instancias). Sin login de demo fijo: se administra desde ahí.",
        creds: [{ label: "apikey (REST)", value: "tmk_evolution_key_2024" }],
      },
    ],
  },
  {
    title: "Analítica",
    services: [
      {
        name: "tabix (SQL UI de ClickHouse)",
        url: "http://localhost:8084",
        login: true,
        desc: "Cliente web para consultar ClickHouse. Conectar usando el host del proxy CORS, no ClickHouse directo (:8123 falla por CORS/login).",
        creds: [
          { label: "host", value: "http://localhost:8085" },
          { label: "usuario", value: "todomark" },
          { label: "password", value: "todomark" },
        ],
      },
      {
        name: "clickhouse-cors-proxy",
        url: "http://localhost:8085",
        login: false,
        desc: "Proxy interno que agrega headers CORS delante de ClickHouse. Es el host que debe usar Tabix (ver arriba), no un panel propio.",
      },
      {
        name: "clickhouse",
        url: "http://localhost:8123",
        login: true,
        desc: "Base analítica. HTTP en :8123, protocolo nativo en :9000. Usar vía Tabix + el proxy CORS, no directo desde el navegador.",
        creds: [
          { label: "usuario", value: "todomark" },
          { label: "password", value: "todomark" },
        ],
      },
      {
        name: "grafana",
        url: "http://localhost:3300",
        login: true,
        desc: "Dashboards sobre ClickHouse. Nota: NO es el puerto 3000 por defecto de Grafana — ese lo ocupa otro proyecto Docker en esta máquina, por eso quedó remapeado a 3300.",
        creds: [
          { label: "usuario", value: "admin" },
          { label: "password", value: "todomark_admin_2026" },
        ],
      },
    ],
  },
  {
    title: "Admin / CMS",
    services: [
      {
        name: "directus",
        url: "http://localhost:8083",
        login: true,
        desc: "CMS admin para el contenido de la landing page.",
        creds: [
          { label: "email", value: "admin@todomark.dev" },
          { label: "password", value: "todomark_admin_2026" },
        ],
      },
    ],
  },
  {
    title: "Backend / infraestructura",
    services: [
      {
        name: "todomark-api",
        url: "http://localhost:8081",
        login: false,
        desc: "API gateway en Go (REST puro, sin UI). Consumida por todamark-web y astro-frontend.",
      },
      {
        name: "hydra (OAuth2 / OIDC)",
        url: "http://localhost:4444",
        login: false,
        desc: "Servidor OAuth2/OIDC. Sin UI de login directa — solo flujo SSO. Puerto público 4444, admin API en 4445 (no expuesto para navegar).",
      },
      {
        name: "postgres",
        login: false,
        desc: "Base de datos interna (Evolution, Hydra, Directus). Sin puerto expuesto al host, sin UI — solo red interna de Docker.",
      },
    ],
  },
];

export default component$(() => {
  return (
    <div class="page">
      <div class="hero">
        <div class="hero-eyebrow">TodoMark · Demo stack</div>
        <h1>Cheat sheet de la demo</h1>
        <p>
          Todas las URLs y credenciales del stack de demo, en un solo lugar.
          Ábrela antes de una demo para no tener que recordar nada.
        </p>
      </div>

      <div class="warning-callout">
        <span class="warning-icon">⚠️</span>
        <div>
          <strong>WhatsApp: un solo número autorizado</strong>
          <span>
            Nunca envíes mensajes de prueba ni reales a ningún número que no
            sea{" "}
            <span class="phone">+51 992 621 314</span>. Esto aplica a
            cualquier prueba manual contra Evolution API o el envío desde
            todomark-api.
          </span>
        </div>
      </div>

      {groups.map((group) => (
        <div class="group" key={group.title}>
          <div class="group-title">{group.title}</div>
          <div class="service-grid">
            {group.services.map((svc) => (
              <div class="card service-card" key={svc.name}>
                <div class="service-card-head">
                  <div class="service-name">{svc.name}</div>
                  <span class={`badge ${svc.login ? "badge-login" : "badge-open"}`}>
                    {svc.login ? "requiere login" : "sin login"}
                  </span>
                </div>

                {svc.tunnelUrl ? (
                  <div class="url-block">
                    <a
                      class="service-url service-url-public"
                      href={svc.tunnelUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      🌐 {svc.tunnelUrl}
                    </a>
                    <span class="service-url-local">
                      💻 local: {svc.url}
                    </span>
                  </div>
                ) : svc.url ? (
                  <div class="url-block">
                    <a
                      class="service-url"
                      href={svc.url}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      {svc.url}
                    </a>
                    <span class="local-only-note">
                      ⚠ solo accesible en la red local del instructor, no expuesto por túnel ahora
                    </span>
                  </div>
                ) : null}

                <p class="service-desc">{svc.desc}</p>

                {svc.routes && (
                  <div class="route-list">
                    {svc.routes.map((r) => (
                      <div class="route-row" key={r.path}>
                        <span class="route-path">{r.path}</span>
                        <span class="route-note">{r.note}</span>
                      </div>
                    ))}
                  </div>
                )}

                {svc.creds && (
                  <div class="cred-list">
                    {svc.creds.map((c) => (
                      <div class="cred-row" key={c.label}>
                        <span class="cred-label">{c.label}</span>
                        <span class="cred-value">{c.value}</span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>

          {group.title === "App interna" && (
            <div class="rbac-note">
              <span>⚑</span>
              <span>
                Los 4 logins de todamark-web son personas de demo, no RBAC
                real: las cuatro cuentas otorgan exactamente el mismo acceso,
                no hay permisos distintos detrás de cada rol.
              </span>
            </div>
          )}
        </div>
      ))}

      <div class="page-footer">
        TodoMark — referencia de demo. Las URLs 🌐 públicas son túneles temporales
        (cloudflared) y pueden dejar de funcionar si se corta la sesión del instructor.
      </div>
    </div>
  );
});

export const head: DocumentHead = {
  title: "TodoMark — Demo Cheat Sheet",
  meta: [
    {
      name: "description",
      content:
        "URLs y credenciales de todos los servicios del stack de demo de TodoMark.",
    },
  ],
};
