# TodoMark

Plataforma de tickets con bot de WhatsApp, clasificación por IA, analítica sobre ClickHouse y dashboards en Grafana.

Todo el stack corre en Docker Compose. No hay modo de despliegue soportado con dev servers locales: `vite dev` y `go run .` sirven solo para iterar, la validación real es en Docker.

---

## 1. Arquitectura

| Servicio | Imagen / build | Puerto host | Rol |
|----------|----------------|-------------|-----|
| `todamark-web` | build `./web` (React + Vite, servido por nginx) | `8080` | Interfaz principal de tickets |
| `todomark-api` | build `./api-go` (Go 1.25) | `8081` → `3000` | Gateway HTTP: tickets, notificaciones, WhatsApp, reportes, IA |
| `evolution` | `evoapicloud/evolution-api:latest` | `3100` → `8080` | Puente con WhatsApp (QR / pairing code) |
| `postgres` | `postgres:16-alpine` | interno | Datos de Evolution, Hydra y Directus |
| `clickhouse` | `clickhouse/clickhouse-server:latest` | `8123`, `9000` | Almacén analítico (tablas `fact_*` / `dim_*`) |
| `clickhouse-cors-proxy` | `nginx:alpine` | `8085` | Añade cabeceras CORS que ClickHouse no pone en la respuesta real |
| `tabix` | `spoonest/clickhouse-tabix-web-client` | `8084` | Cliente SQL web para ClickHouse |
| `grafana` | `grafana/grafana-oss:latest` | `3300` → `3000` | Dashboards, datasource y panel provisionados como código |
| `hydra` | `oryd/hydra:v2.2.0` | `4444` (público), `4445` (admin) | OAuth2 / OIDC |
| `hydra-migrate` | `oryd/hydra:v2.2.0` | — | Job de migración, corre una vez y termina |
| `directus` | `directus/directus:latest` | `8083` → `8055` | CMS headless del contenido de la landing |
| `astro-frontend` | build `./astro-frontend` | `8082` | Landing pública (estática, contenido desde Directus en build time) |
| `qwik-frontend` | build `./qwik-frontend` | `8086` | Frontend experimental |

Nota sobre nombres: el servicio web es `todamark-web` (con «a»), el de API es `todomark-api`. No es un error tipográfico del README; es el nombre real en `docker-compose.yml` y los comandos deben usarlo tal cual.

**Persistencia.** Todos los datos viven en volúmenes Docker con nombre (`sqlite_data`, `postgres_data`, `evolution_instances`, `clickhouse_data`, `grafana_data`), nunca dentro del repositorio. Las conversaciones de WhatsApp quedan en `postgres_data` y `evolution_instances`. Los `.db` y `.sqlite` están en `.gitignore` y no se versionan.

---

## 2. Requisitos previos

- Docker Engine 24+ con el plugin Compose v2 (`docker compose`, sin guion).
- 4 GB de RAM libres como mínimo: ClickHouse y Grafana son los servicios pesados.
- Los puertos `8080`, `8081`, `8082`, `8083`, `8084`, `8085`, `8086`, `3100`, `3300`, `4444`, `4445`, `8123` y `9000` libres en el host.
- Un teléfono con WhatsApp si se va a usar el bot (hay que escanear un QR o pedir un pairing code).

Para compilar fuera de Docker: Node 20+ y Go 1.25+.

---

## 3. Despliegue desde cero

### Paso 1 — Clonar y crear el `.env`

```bash
git clone https://github.com/davrv93/todomark.git
cd todomark
cp .env.example .env
```

Editar `.env`. Como mínimo hay que definir `EVOLUTION_API_KEY` con un valor propio y ajustar `VITE_API_URL` si el despliegue no es en `localhost`. El detalle de cada variable está en la sección 5.

`.env` está en `.gitignore`. No debe subirse nunca al repositorio.

### Paso 2 — Levantar Postgres y crear las bases de datos de Hydra y Directus

La imagen de Postgres solo crea la base `todomark` (la que usa Evolution). Hydra y Directus apuntan a bases `hydra` y `directus` que **no** se crean solas y no hay script de inicialización en el repositorio. Hay que crearlas antes del primer arranque completo, o `hydra-migrate` y `directus` fallarán al iniciar.

```bash
docker compose up -d postgres

# Esperar a que Postgres acepte conexiones
until docker compose exec -T postgres pg_isready -U todomark; do sleep 2; done

docker compose exec -T postgres psql -U todomark -d todomark -c "CREATE DATABASE hydra;"
docker compose exec -T postgres psql -U todomark -d todomark -c "CREATE DATABASE directus;"
```

Si las bases ya existen, `psql` responde `ERROR: database "hydra" already exists`. Ese error es inofensivo y se puede ignorar.

### Paso 3 — Construir y levantar el stack

```bash
docker compose up -d --build
```

La primera construcción tarda varios minutos: compila el binario de Go, tres frontends y descarga las imágenes de ClickHouse, Grafana, Hydra y Directus.

### Paso 4 — Verificar que todo está arriba

```bash
docker compose ps
```

Todos los servicios deben aparecer `Up` (los que tienen healthcheck, `Up (healthy)`). La única excepción esperada es `hydra-migrate`, que aparece `Exited (0)`: es un job de migración que corre una vez y termina correctamente.

Comprobaciones de humo:

```bash
curl -fsS http://localhost:8081/healthz   # API Go
curl -fsS http://localhost:8080/healthz   # Web
curl -fsS http://localhost:8123/ping      # ClickHouse
curl -fsS http://localhost:4445/health/ready  # Hydra admin
```

El esquema analítico de ClickHouse (`fact_tickets`, `fact_events`, `fact_notifications`, `dim_client`, `dim_user`, `dim_time`) lo crea el propio `todomark-api` al arrancar, con `CREATE TABLE IF NOT EXISTS`. No hay que ejecutar migraciones a mano.

### Paso 5 — Vincular WhatsApp

Con el stack arriba, abrir `http://localhost:8080` e ir a la vista de WhatsApp. También se puede hacer por API:

```bash
curl -fsS  http://localhost:8081/api/whatsapp/status   # estado de la sesión
curl -fsSX POST http://localhost:8081/api/whatsapp/qr  # genera el QR a escanear
curl -fsSX POST http://localhost:8081/api/whatsapp/pair \
     -H 'Content-Type: application/json' \
     -d '{"number":"51999999999"}'                      # alternativa: pairing code
curl -fsSX POST http://localhost:8081/api/whatsapp/logout
```

Escanear el QR desde WhatsApp → Dispositivos vinculados. La sesión queda persistida en el volumen `evolution_instances` y sobrevive a los reinicios.

**Candado de destinatarios.** La variable `WHATSAPP_ALLOWLIST` limita a qué números puede escribir el bot. Por defecto trae tres números autorizados. Cualquier envío a un número fuera de la lista se rechaza. Revisar y ajustar esta lista antes de exponer el sistema: es la protección contra envíos accidentales a terceros.

### Paso 6 — Registrar el cliente OAuth en Hydra (opcional)

Solo hace falta si se quiere login por OIDC. Sin esto, el frontend cae al login local de usuario y contraseña, que es el comportamiento por defecto.

```bash
docker compose exec hydra hydra create oauth2-client \
  --endpoint http://127.0.0.1:4445 \
  --name todomark-web \
  --grant-type authorization_code,refresh_token \
  --response-type code \
  --scope openid,offline_access,admin \
  --redirect-uri http://localhost:8080/ \
  --post-logout-callback http://localhost:8080/login \
  --token-endpoint-auth-method none
```

El comando devuelve un `CLIENT ID`. Copiarlo al `.env`:

```bash
VITE_HYDRA_CLIENT_ID=<el client id devuelto>
VITE_HYDRA_AUTH_URL=http://localhost:4444/
```

Estas dos variables se inyectan como **build args**, no como entorno de runtime. Después de cambiarlas hay que reconstruir el frontend:

```bash
docker compose up -d --build todamark-web astro-frontend
```

### Paso 7 — Sembrar el contenido de la landing en Directus (opcional)

Solo hace falta si se usa `astro-frontend`. Entrar a `http://localhost:8083` con las credenciales de `DIRECTUS_ADMIN_EMAIL` / `DIRECTUS_ADMIN_PASSWORD`, crear la colección `landing_content` con campos `key` y `value`, darle lectura pública y cargar el contenido.

La landing es **estática**: consume Directus en build time, no en runtime. Editar contenido en Directus no se refleja con un `up -d --build` normal, porque Docker cachea la capa de build. Para publicar cambios de contenido:

```bash
docker compose build --no-cache astro-frontend
docker compose up -d astro-frontend
```

Si Directus no responde durante el build, Astro usa un texto de respaldo incrustado en `landing.astro` y el build no falla. Esto puede dar la falsa impresión de que el contenido se publicó: verificar el HTML servido, no solo que el build haya terminado.

---

## 4. Accesos por defecto

| Servicio | URL | Credenciales |
|----------|-----|--------------|
| Web (tickets) | http://localhost:8080 | Login local `admin` / `user` |
| Landing | http://localhost:8082 | — |
| API | http://localhost:8081 | — |
| Directus | http://localhost:8083 | `DIRECTUS_ADMIN_EMAIL` / `DIRECTUS_ADMIN_PASSWORD` |
| Tabix (SQL) | http://localhost:8084 | Servidor `http://localhost:8123`, usuario y contraseña `todomark` |
| Grafana | http://localhost:3300 | `admin` / `GRAFANA_ADMIN_PASSWORD` |
| Evolution API | http://localhost:3100 | Cabecera `apikey: $EVOLUTION_API_KEY` |

---

## 5. Variables de entorno

Todas se definen en `.env` en la raíz del proyecto.

### Obligatorias

| Variable | Descripción |
|----------|-------------|
| `EVOLUTION_API_KEY` | Clave de autenticación de Evolution API. Poner un valor propio, no el del ejemplo |
| `VITE_API_URL` | URL pública de la API tal como la ve el navegador. Por defecto `http://localhost:8081` |

### Base de datos analítica

| Variable | Por defecto | Descripción |
|----------|-------------|-------------|
| `CLICKHOUSE_USER` | `todomark` | Usuario de ClickHouse |
| `CLICKHOUSE_PASSWORD` | `todomark` | Contraseña de ClickHouse |
| `CLICKHOUSE_DB` | `todomark` | Base analítica |

### Proveedores de IA

| Variable | Descripción |
|----------|-------------|
| `GEMINI_API_KEY` | Clave de Google Gemini. Sin ella, la clasificación y el enrutado de intención por IA quedan desactivados |
| `GEMINI_API_URL` | Endpoint del modelo. Por defecto `gemini-3.5-flash-lite` |
| `DEEPSEEK_API_KEY` | Clave de DeepSeek |
| `DEEPSEEK_API_URL` | Por defecto `https://api.deepseek.com/v1/chat/completions` |

### WhatsApp

| Variable | Descripción |
|----------|-------------|
| `WHATSAPP_ALLOWLIST` | Lista de números autorizados, separados por comas, en dígitos con código de país. Único destino permitido para los envíos |
| `EVOLUTION_INSTANCE` | Nombre de la instancia de Evolution. Por defecto `todomark` |
| `USE_META_WHATSAPP` | `true` para usar la Cloud API de Meta en lugar de Evolution |
| `META_WHATSAPP_TOKEN`, `META_WHATSAPP_PHONE_NUMBER_ID`, `META_WHATSAPP_TEMPLATE`, `META_WHATSAPP_LANGUAGE` | Configuración de la Cloud API de Meta, solo si `USE_META_WHATSAPP=true` |

### Autenticación

| Variable | Descripción |
|----------|-------------|
| `REQUIRE_API_AUTH` | `true` exige token en todas las rutas de la API |
| `API_BEARER_TOKEN` | Token estático, alternativa simple a Hydra |
| `VITE_HYDRA_CLIENT_ID`, `VITE_HYDRA_AUTH_URL` | Cliente OIDC del frontend. **Build args**: al cambiarlos hay que reconstruir |
| `HYDRA_SYSTEM_SECRET` | Secreto de sistema de Hydra, mínimo 32 bytes. Cambiar obligatoriamente en producción |
| `HYDRA_INTROSPECTION_URL`, `HYDRA_CLIENT_ID`, `HYDRA_CLIENT_SECRET` | Validación de tokens del lado de la API |

### Integraciones opcionales

| Variable | Descripción |
|----------|-------------|
| `GLPI_URL`, `GLPI_APP_TOKEN`, `GLPI_USER`, `GLPI_PASSWORD` | Sincronización con GLPI |
| `EMAIL_WEBHOOK_URL`, `EMAIL_API_KEY` | Notificaciones por correo |
| `GRAFANA_ADMIN_PASSWORD` | Contraseña de administrador de Grafana |
| `DIRECTUS_KEY`, `DIRECTUS_SECRET`, `DIRECTUS_ADMIN_EMAIL`, `DIRECTUS_ADMIN_PASSWORD` | Configuración de Directus |

---

## 6. Inteligencia artificial

### En ejecución

El producto llama a dos proveedores desde `todomark-api`:

- **Google Gemini** (`api-go/internal/gemini/`) — clasifica los mensajes entrantes de WhatsApp, enruta la intención del usuario (menú, incidencia, reporte o pregunta) y genera las respuestas conversacionales del bot.
- **DeepSeek** (`api-go/internal/deepseek/`) — motor del chat de reportes (`api-go/internal/reportchat/`), que responde preguntas en lenguaje natural sobre los datos analíticos.

Ambos son opcionales: si falta la clave de API, el servicio arranca igual y esas funciones quedan inactivas. El código maneja los errores de cuota de Gemini con un reintento automático.

### En el desarrollo

Este proyecto se construyó con asistencia de varias herramientas de IA: **Claude**, **Muse** y **DeepSeek**. Todo el código generado con su ayuda fue revisado y verificado antes de integrarse.

---

## 7. Verificación de compilación

Antes de dar por cerrado cualquier cambio (Vite no comprueba tipos por su cuenta):

```bash
# Frontend
cd web && npx tsc --noEmit && npm run build

# API
cd api-go && go build ./... && go test ./...
```

Y la verificación de extremo a extremo, que es la que manda:

```bash
docker compose up -d --build
docker compose ps
docker compose logs --tail=50 todomark-api todamark-web
```

---

## 8. Operación

**Aplicar cambios de código.** Un `restart` no basta: hay que reconstruir la imagen.

```bash
docker compose up -d --build todomark-api     # o el servicio que se tocó
```

**Ver logs.**

```bash
docker compose logs -f todomark-api
docker compose logs -f todamark-web
docker compose logs -f evolution
```

**Parar el stack conservando los datos.**

```bash
docker compose down
```

> **Aviso: destructivo e irreversible.**
> El siguiente comando borra los volúmenes con nombre. Se pierden los tickets, el historial de conversaciones de WhatsApp, la sesión vinculada del teléfono, los datos analíticos de ClickHouse y la configuración de Grafana y Directus. No hay forma de deshacerlo sin una copia de seguridad previa.
>
> ```bash
> docker compose down -v
> ```
>
> Hacer copia de seguridad antes de ejecutarlo.

**Copia de seguridad de Postgres** (contiene las conversaciones de WhatsApp):

```bash
docker compose exec -T postgres pg_dumpall -U todomark > backup-$(date +%F).sql
```

---

## 9. Notas de seguridad para producción

Los valores por defecto de `docker-compose.yml` son de desarrollo local. Antes de exponer el sistema fuera de una máquina personal hay que cambiar, como mínimo:

- `POSTGRES_PASSWORD`, fijo en `todomark` dentro de `docker-compose.yml`.
- La contraseña del datasource en `grafana/provisioning/datasources/clickhouse.yaml`, también `todomark` en claro. Moverla a una variable de entorno.
- `HYDRA_SYSTEM_SECRET`, `DIRECTUS_KEY` y `DIRECTUS_SECRET`, que traen valores de ejemplo con `change_me` en el nombre.
- `GRAFANA_ADMIN_PASSWORD` y `DIRECTUS_ADMIN_PASSWORD`.
- `EVOLUTION_API_KEY`, que no debe quedarse con el valor de `.env.example`.

Además: Hydra arranca con `serve all --dev`, que desactiva la exigencia de HTTPS. Para producción hay que quitar ese flag y poner el stack detrás de TLS. Y el login local de `admin` / `user` está codificado en el frontend; conviene desactivarlo en favor de OIDC.

---

## 10. Documentación relacionada

- `DOCUMENTACION.md` — arquitectura, decisiones de diseño y detalle funcional.
- `PLAN.md` — plan de trabajo por fases y estado de avance.
- `AGENTS.md` — convenciones para agentes de IA que trabajen en este repositorio.
