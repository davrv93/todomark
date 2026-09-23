// @ts-check
import { defineConfig } from 'astro/config';

import react from '@astrojs/react';

// Astro compila sobre Vite, pero a diferencia de un proyecto Vite puro solo expone al
// cliente (import.meta.env) las variables prefijadas con PUBLIC_, no las prefijadas VITE_.
// Por eso este proyecto usa PUBLIC_API_URL / PUBLIC_HYDRA_CLIENT_ID / PUBLIC_HYDRA_AUTH_URL
// (ver src/services/ticketService.ts) en vez de los nombres VITE_* que usa web/. No se
// necesita configuración adicional aquí: basta con pasarlas como ENV en build time
// (ver Dockerfile) para que Vite las inline durante `astro build`.
// https://astro.build/config
export default defineConfig({
  integrations: [react()],
  output: 'static',
});
