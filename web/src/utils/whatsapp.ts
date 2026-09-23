// Individual: solo dígitos (código de país + número, sin +).
// Grupo: JID de WhatsApp terminado en @g.us (Baileys/Evolution).
export const WHATSAPP_TARGET_RE = /^\d{8,15}$|^\d{10,30}(-\d{1,15})?@g\.us$/;

export function isGroupTarget(value: string): boolean {
  return value.endsWith('@g.us');
}
