// Package report arma el resumen ejecutivo (mismos datos de GET /api/reports/executive)
// como texto plano, PDF, Excel y gráfico, para los comandos del bot de WhatsApp.
package report

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"

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

// Excel arma el mismo resumen ejecutivo como .xlsx (misma ExecutiveSummary que Text/PDF:
// ningún número se calcula distinto acá).
func Excel(s store.ExecutiveSummary) ([]byte, error) {
	rows := [][]string{
		{"Tickets abiertos", fmt.Sprintf("%d", s.OpenNow)},
		{"Cambio neto (7 dias)", fmt.Sprintf("%+d", s.NetChange7d)},
		{"MTTR (horas)", fmtHours(s.MTTRHours)},
		{"Tasa de reapertura", fmtPct(s.ReopenRatePct)},
		{"CSAT (satisfaccion)", func() string {
			if s.CSATAvg == nil {
				return "sin datos"
			}
			return fmt.Sprintf("%.1f / 5", *s.CSATAvg)
		}()},
		{"% cumplimiento SLA (demo)", fmtPct(s.SLACompliancePct)},
		{"Costo estimado de soporte (demo)", func() string {
			if s.CostEstimateDemo == nil {
				return "sin datos"
			}
			return fmt.Sprintf("$%.0f", *s.CostEstimateDemo)
		}()},
		{"Visitas a la landing", fmt.Sprintf("%d", s.LandingVisits)},
		{"Leads capturados", fmt.Sprintf("%d", s.LeadsTotal)},
	}
	return TableXLSX("Resumen ejecutivo", []string{"Indicador", "Valor"}, rows)
}

// TableXLSX vuelca columnas/filas a una hoja de Excel. Lo usan tanto Excel() como el bot
// cuando el modelo devuelve una tabla en la respuesta.
func TableXLSX(sheet string, columns []string, rows [][]string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	if sheet == "" {
		sheet = "Datos"
	}
	if len(sheet) > 31 {
		sheet = sheet[:31]
	}
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(idx)
	f.DeleteSheet("Sheet1")

	write := func(rowNum int, values []string) error {
		for i, v := range values {
			cell, err := excelize.CoordinatesToCellName(i+1, rowNum)
			if err != nil {
				return err
			}
			if err := f.SetCellStr(sheet, cell, v); err != nil {
				return err
			}
		}
		return nil
	}
	if err := write(1, columns); err != nil {
		return nil, err
	}
	for i, r := range rows {
		if err := write(i+2, r); err != nil {
			return nil, err
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// TableText renderiza una tabla como bloque monoespaciado de WhatsApp (```), para las
// respuestas que no piden archivo.
func TableText(columns []string, rows [][]string) string {
	widths := make([]int, len(columns))
	for i, c := range columns {
		widths[i] = len(c)
	}
	for _, r := range rows {
		for i, cell := range r {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	line := func(cells []string) string {
		parts := make([]string, 0, len(widths))
		for i := range widths {
			v := ""
			if i < len(cells) {
				v = cells[i]
			}
			parts = append(parts, v+strings.Repeat(" ", widths[i]-len(v)))
		}
		return strings.TrimRight(strings.Join(parts, "  "), " ")
	}
	var sb strings.Builder
	sb.WriteString("```\n")
	sb.WriteString(line(columns) + "\n")
	for _, r := range rows {
		sb.WriteString(line(r) + "\n")
	}
	sb.WriteString("```")
	return sb.String()
}

// ChartPDF dibuja un gráfico de barras horizontales con fpdf (la misma librería del PDF
// ejecutivo — sin dependencias nuevas). line/doughnut se dibujan igual como barras: es el
// mismo dato, cambia la forma, y se aclara en el título.
func ChartPDF(title, chartType string, labels []string, values []float64) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	if title == "" {
		title = "TodoMark - Grafico"
	}
	pdf.CellFormat(0, 12, title, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(110, 110, 115)
	pdf.CellFormat(0, 6, fmt.Sprintf("Generado %s - %d valores (%s)", time.Now().Format("02/01/2006 15:04"), len(values), chartType), "", 1, "L", false, 0, "")
	pdf.Ln(4)
	pdf.SetTextColor(0, 0, 0)

	max := 0.0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	if max <= 0 {
		max = 1
	}
	const barMaxW = 110.0
	for i, v := range values {
		label := ""
		if i < len(labels) {
			label = labels[i]
		}
		if len(label) > 28 {
			label = label[:28]
		}
		y := pdf.GetY()
		pdf.SetFont("Helvetica", "", 10)
		pdf.CellFormat(50, 7, label, "", 0, "L", false, 0, "")
		pdf.SetFillColor(59, 110, 245)
		w := barMaxW * v / max
		if w > 0 {
			pdf.Rect(pdf.GetX(), y+1.5, w, 4, "F")
		}
		pdf.SetX(pdf.GetX() + barMaxW + 2)
		pdf.CellFormat(0, 7, fmt.Sprintf("%.1f", v), "", 1, "L", false, 0, "")
		if pdf.GetY() > 265 {
			pdf.AddPage()
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
