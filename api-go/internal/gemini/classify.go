package gemini

import "strings"

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
