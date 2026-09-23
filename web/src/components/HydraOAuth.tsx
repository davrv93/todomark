import React from 'react';
import { AuthProvider, AuthProviderProps } from 'react-oidc-context';

export const HYDRA_AUTH_URL: string = import.meta.env.VITE_HYDRA_AUTH_URL ?? '';
export const HYDRA_CLIENT_ID: string = import.meta.env.VITE_HYDRA_CLIENT_ID ?? '';

/** true cuando Hydra está configurado (authority + client_id); false = usar el login local de siempre. */
export const isHydraConfigured = Boolean(HYDRA_AUTH_URL && HYDRA_CLIENT_ID);

const oidcConfig: AuthProviderProps = {
  authority: HYDRA_AUTH_URL,
  client_id: HYDRA_CLIENT_ID,
  redirect_uri: `${window.location.origin}/`,
  scope: 'openid offline_access admin',
  onSigninCallback: () => {
    // Limpia code/state de la URL tras volver del login (patrón estándar de react-oidc-context).
    window.history.replaceState({}, document.title, window.location.pathname);
  },
};

export default function HydraOAuth({ children }: { children: React.ReactNode }) {
  if (!isHydraConfigured) {
    // Sin authority configurada: no montar AuthProvider (evita inicializar oidc-client-ts contra una URL vacía).
    return <>{children}</>;
  }
  return <AuthProvider {...oidcConfig}>{children}</AuthProvider>;
}
