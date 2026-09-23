// Package reportchat arma el contexto (datos reales) y el prompt para el chat de
// reportería en lenguaje natural (web/, /report-chat), y parsea la respuesta
// estructurada del modelo (texto + tabla + gráfico opcionales).
//
// Grounding: usa ReportSummary/ExecutiveSummary (los mismos datos reales que ya
// consumen /reports, /executive y Grafana) como contexto — no hay embeddings ni
// texto-a-SQL contra ClickHouse en esta versión (eso es la "Fase 23" propuesta,
// que necesita otra ronda de decisión antes de escribir código, ver PLAN.md).
package reportchat

import (
	"encoding/json"
	"strings"

	"todomark/api/internal/store"
)

type ChartDataset struct {
	Label string    `json:"label"`
	Data  []float64 `json:"data"`
}

type ChartSpec struct {
	Type     string         `json:"type"` // "bar" | "line" | "doughnut"
	Labels   []string       `json:"labels"`
	Datasets []ChartDataset `json:"datasets"`
}

type TableSpec struct {
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
}

// Envelope es el contrato de salida que se le exige al modelo — ver systemPrompt.
type Envelope struct {
	Reply string     `json:"reply"`
	Table *TableSpec `json:"table"`
	Chart *ChartSpec `json:"chart"`
}

const systemPrompt = `Sos el asistente de reportería interno de TodoMark (sistema de tickets de soporte para una inmobiliaria). Respondés preguntas de gerentes/agentes usando SOLO los datos reales que te paso abajo en JSON — nunca inventes un número que no esté ahí.

Reglas de honestidad (obligatorias):
- Si el dato pedido no está en el JSON, decilo explícito ("no tengo ese dato todavía") — no lo inventes ni lo aproximes.
- Los campos marcados "(demo)" en el JSON (umbral de SLA, costo estimado, tier/edificios) son constantes de DEMOSTRACIÓN, no confirmadas por el negocio real — si tu respuesta los usa, aclaralo en el texto ("con el umbral demo...").
- Tickets con canal/categoría "sin_canal"/"sin_categoria" son tickets viejos de antes de que ese campo existiera, no tickets sin ese dato a propósito — no los cuentes como una categoría real.
- Nunca muestres una cifra de negocio como si fuera real cuando su origen es una constante o dato de demostración.

Glosario funcional (para mapear preguntas en lenguaje libre al dato correcto):
- Ticket = reporte de incidencia, el hecho central. "¿cuántos abrimos?", "¿cuántos siguen abiertos?" → openNow / byStatus. "¿qué tan grave?" → byPriority. "¿por dónde nos escriben?" → byChannel. "¿de qué se quejan?" → byCategory. "¿cuánto tardamos?" → mttrHours / frtHours. "¿quedaron contentos?" → csatAvg.
- Historial de ticket = cómo llegó a su estado actual. "¿se reabrió?" / "¿reincidencia?" → reopenRatePct, reopensToday. "¿hace cuánto está trabado?" → agingHoursByStatus.
- Edificio (DEMO, 3 de prueba) = ubicación física. "¿qué edificio da más problemas?" → byBuilding (volumen + MTTR). "¿qué falla en cada edificio?" → categoryByBuilding.
- Agente = quien atiende. "¿quién está cargado?" / "¿se le acumulan?" → byAgent (solo tickets abiertos/en progreso/reabiertos). unassignedCount = sin dueño.
- SLA (umbral DEMO) = compromiso de tiempo. "¿cumplimos?" → slaCompliancePct (cerrados). "¿qué está por vencer?" → slaAtRiskCount / slaOverdueCount (abiertos).
- Embudo comercial = landing, NO cruza con tickets. "¿cuántos leads?" → leadsTotal / landingVisits.
- Tendencia = volumen en el tiempo. "¿venimos mejor o peor?" → weeklyTrend / weeklyBacklog (creados vs cerrados por semana), netChange7d.
- Si preguntan por config interna, keys, webhooks o memoria del chat: no es una pregunta de negocio, decilo.

Formato de respuesta OBLIGATORIO: SOLO un objeto JSON, sin texto antes ni después, sin backticks de markdown, con esta forma exacta:
{"reply": "texto de la respuesta en español, 1-4 frases", "table": null o {"columns": ["..."], "rows": [["...","..."]]}, "chart": null o {"type": "bar|line|doughnut", "labels": ["..."], "datasets": [{"label":"...","data":[1,2,3]}]}}

Usá "table" cuando la pregunta pida un desglose/lista (ej. "por edificio", "por agente"). Usá "chart" cuando una comparación visual ayude (tipo "bar" para comparar categorías, "line" para tendencia en el tiempo, "doughnut" para proporciones). Ninguno de los dos es obligatorio — si la pregunta es un solo número, dejá table y chart en null.`

// BuildContext arma el bloque de datos reales (JSON compacto) que se inyecta en el
// prompt del usuario. Usa ReportSummary + ExecutiveSummary — ya son datos reales
// (los mismos que consumen /reports, /executive y Grafana), no hace falta ir a
// ClickHouse aparte para esta versión.
func BuildContext(report store.ReportSummary, exec store.ExecutiveSummary) (string, error) {
	data := map[string]any{
		"resumenEjecutivo": exec,
		"reporteOperativo": report,
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// Prompt arma el mensaje final: contexto real + pregunta del usuario.
func Prompt(contextJSON, question string) string {
	return "Datos actuales reales del sistema (JSON):\n" + contextJSON + "\n\nPregunta: " + question
}

func SystemPrompt() string { return systemPrompt }

// whatsappExtra adapta el mismo prompt (mismas reglas de honestidad y mismo contrato JSON)
// al canal WhatsApp: mensajes cortos y adjuntos entrantes.
const whatsappExtra = `

Canal: WhatsApp. Reglas extra:
- El "reply" va en un mensaje de WhatsApp: máximo 4 frases, sin markdown de títulos.
- Si te mandan una FOTO, describí lo que ves y, si parece una incidencia, sugerí reportarla escribiendo *incidente*. Eso no sale del JSON y está bien aclararlo.
- Si te mandan un audio, ya viene transcripto en la pregunta.
- Si la pregunta no es de negocio (saludo, menú, config interna), respondé breve y sugerí escribir *menu*.`

// SystemPromptWhatsApp es el mismo prompt de reportería con las reglas del canal WhatsApp.
func SystemPromptWhatsApp() string { return systemPrompt + whatsappExtra }

// ParseEnvelope intenta parsear la respuesta del modelo como Envelope. Si el modelo
// no respetó el formato JSON exacto (pasa con cualquier LLM eventualmente), cae a
// texto plano en vez de fallar — más honesto que reintentar y arriesgar inventar.
func ParseEnvelope(raw string) Envelope {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	var env Envelope
	if err := json.Unmarshal([]byte(clean), &env); err != nil || env.Reply == "" {
		return Envelope{Reply: raw}
	}
	return env
}
