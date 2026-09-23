// Package report arma el resumen ejecutivo (mismos datos de GET /api/reports/executive)
// como texto plano y como PDF, para el comando "reporte" del bot de WhatsApp.
package report

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"

	"todomark/api/internal/store"
)

func fmtHours(h *float64) string {
	if h == nil {
		return "sin datos"
	}
	return fmt.Sprintf("%.1f h", *h)
}

func fmtPct(p *float64) string {
	if p == nil {
		return "sin datos"
	}
	return fmt.Sprintf("%.1f%%", *p)
}

// Text arma el mensaje de WhatsApp con los mismos KPIs del dashboard ejecutivo.
// Ningún número se inventa acá: viene de la misma ExecutiveSummary que ya usa /executive.
func Text(s store.ExecutiveSummary) string {
	return "📊 *Reporte ejecutivo TodoMark*\n" +
		"_" + time.Now().Format("02/01/2006 15:04") + "_\n\n" +
		fmt.Sprintf("🎫 Tickets abiertos: *%d*\n", s.OpenNow) +
		fmt.Sprintf("📈 Cambio neto (7d): *%+d*\n", s.NetChange7d) +
		fmt.Sprintf("⏱ MTTR: *%s*\n", fmtHours(s.MTTRHours)) +
		fmt.Sprintf("🔁 Tasa de reapertura: *%s*\n", fmtPct(s.ReopenRatePct)) +
		fmt.Sprintf("⭐ CSAT: *%s*\n", func() string {
			if s.CSATAvg == nil {
				return "sin datos"
			}
			return fmt.Sprintf("%.1f / 5", *s.CSATAvg)
		}()) +
		fmt.Sprintf("🎯 %% SLA (demo): *%s*\n", fmtPct(s.SLACompliancePct)) +
		fmt.Sprintf("💰 Costo estimado (demo): *%s*\n", func() string {
			if s.CostEstimateDemo == nil {
				return "sin datos"
			}
			return fmt.Sprintf("$%.0f", *s.CostEstimateDemo)
		}()) +
		fmt.Sprintf("🌐 Visitas → Leads: *%d → %d*\n\n", s.LandingVisits, s.LeadsTotal) +
		"_PDF adjunto con el detalle. Datos demo etiquetados como tal, ver DOCUMENTACION.md §13._"
}

// PDF arma una página simple con los mismos KPIs (sin gráficos: fpdf no los necesita para
// esto, es un resumen de texto/tabla — suficiente para lo pedido, sin sumar complejidad).
func PDF(s store.ExecutiveSummary) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 18)
	pdf.CellFormat(0, 12, "TodoMark - Reporte Ejecutivo", "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(110, 110, 115)
	pdf.CellFormat(0, 8, "Generado "+time.Now().Format("02/01/2006 15:04"), "", 1, "L", false, 0, "")
	pdf.Ln(4)
	pdf.SetTextColor(0, 0, 0)

	row := func(label, value string) {
		pdf.SetFont("Helvetica", "", 11)
		pdf.CellFormat(90, 9, label, "B", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "B", 11)
		pdf.CellFormat(0, 9, value, "B", 1, "L", false, 0, "")
	}

	optH := func(h *float64) string {
		if h == nil {
			return "Sin datos"
		}
		return fmt.Sprintf("%.1f horas", *h)
	}
	optPct := func(p *float64) string {
		if p == nil {
			return "Sin datos"
		}
		return fmt.Sprintf("%.1f%%", *p)
	}

	row("Tickets abiertos", fmt.Sprintf("%d", s.OpenNow))
	row("Cambio neto (7 dias)", fmt.Sprintf("%+d", s.NetChange7d))
	row("MTTR (tiempo medio de resolucion)", optH(s.MTTRHours))
	row("Tasa de reapertura", optPct(s.ReopenRatePct))
	if s.CSATAvg == nil {
		row("CSAT (satisfaccion)", "Sin datos")
	} else {
		row("CSAT (satisfaccion)", fmt.Sprintf("%.1f / 5", *s.CSATAvg))
	}
	row("% cumplimiento SLA (demo)", optPct(s.SLACompliancePct))
	if s.CostEstimateDemo == nil {
		row("Costo estimado de soporte (demo)", "Sin datos")
	} else {
		row("Costo estimado de soporte (demo)", fmt.Sprintf("$%.0f", *s.CostEstimateDemo))
	}
	row("Visitas a la landing", fmt.Sprintf("%d", s.LandingVisits))
	row("Leads capturados", fmt.Sprintf("%d", s.LeadsTotal))

	pdf.Ln(10)
	pdf.SetFont("Helvetica", "I", 9)
	pdf.SetTextColor(110, 110, 115)
	pdf.MultiCell(0, 5, "Nota: %SLA y costo estimado usan umbrales/tarifa DEMO, no confirmados por un "+
		"gerente real. Ver DOCUMENTACION.md secciones 13-14 para el detalle de que dato es real y cual es demo.", "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
