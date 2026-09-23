package gemini

import "strings"

// Intent es la intención ruteada de un mensaje entrante del bot de WhatsApp.
type Intent string

const (
	IntentMenu     Intent = "menu"      // navegar el menú / opciones del ticket
	IntentIncident Intent = "incidente" // reportar un problema nuevo (crea ticket)
	IntentReport   Intent = "reporte"   // pedir reporte/KPIs (texto, PDF, Excel, gráfico)
	IntentQuestion Intent = "pregunta"  // pregunta abierta sobre los datos
)

// routePrompt: mismas 4 intenciones que maneja el bot, una palabra de respuesta.
const routePrompt = "Clasifica el mensaje de un usuario de un sistema de tickets de soporte en exactamente una palabra de esta lista: menu, incidente, reporte, pregunta.\n" +
	"- menu: saluda, pide opciones o quiere navegar el menú.\n" +
	"- incidente: describe un problema/avería que quiere reportar.\n" +
	"- reporte: pide un reporte, resumen, KPIs, PDF, Excel o gráfico.\n" +
	"- pregunta: cualquier otra consulta sobre datos, tickets o el servicio.\n" +
	"Responde solo la palabra."

var validIntents = map[Intent]bool{
	IntentMenu: true, IntentIncident: true, IntentReport: true, IntentQuestion: true,
}

// edgeCommands son los comandos exactos que se resuelven localmente (sin red, sin cuota) —
// la parte "edge" del clasificador. Se comparan contra el mensaje completo, no como
// substring: así "pasame en excel el desglose por categoría" sigue siendo una pregunta
// abierta y no el comando "excel".
var edgeCommands = map[string]Intent{
	"menu": IntentMenu, "menú": IntentMenu, "hola": IntentMenu, "buenas": IntentMenu,
	"opciones": IntentMenu, "ayuda": IntentMenu, "start": IntentMenu,

	"reporte": IntentReport, "reportes": IntentReport, "resumen": IntentReport,
	"pdf": IntentReport, "excel": IntentReport, "xlsx": IntentReport,
	"grafico": IntentReport, "gráfico": IntentReport, "kpi": IntentReport,

	"incidente": IntentIncident, "problema": IntentIncident, "reportar": IntentIncident,
	"nuevo ticket": IntentIncident,
}

// RouteIntent rutea la intención del mensaje: primero reglas locales baratas; si ninguna
// matchea y hay API key, le pregunta al modelo; si el modelo falla o no está configurado,
// devuelve fallback (el comportamiento que el bot ya tenía antes de existir el clasificador).
func (c *Client) RouteIntent(text string, fallback Intent) Intent {
	msg := strings.ToLower(strings.TrimSpace(text))
	msg = strings.Trim(msg, ".,!¡?¿*_ ")
	if msg == "" {
		return fallback
	}
	if intent, ok := edgeCommands[msg]; ok {
		return intent
	}
	if !c.Configured() {
		return fallback
	}
	reply, err := c.Chat(routePrompt, msg)
	if err != nil {
		return fallback
	}
	intent := Intent(strings.Trim(strings.ToLower(strings.TrimSpace(reply)), ".,! "))
	if !validIntents[intent] {
		return fallback
	}
	return intent
}

// classifyPrompt le pide al LLM una categoría entre 4 opciones fijas — es clasificación
// real vía prompt (no un modelo de ML entrenado aparte).
const classifyPrompt = "Clasifica el siguiente mensaje en exactamente una palabra de esta lista: demo, soporte, queja, otro. Responde solo la palabra."

var validCategories = map[string]bool{
	"demo":    true,
	"soporte": true,
	"queja":   true,
	"otro":    true,
}

// Classify pide al LLM que categorice message en una de 4 categorías fijas.
// Si la respuesta no matchea ninguna categoría conocida, devuelve "otro".
func (c *Client) Classify(message string) (string, error) {
	reply, err := c.Chat(classifyPrompt, message)
	if err != nil {
		return "", err
	}
	category := strings.ToLower(strings.TrimSpace(reply))
	category = strings.Trim(category, ".,! ")
	if !validCategories[category] {
		return "otro", nil
	}
	return category, nil
}
