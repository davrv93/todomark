// Programa standalone de seed — NO se compila en la imagen Docker de producción (el
// Dockerfile solo hace `go build -o /bin/api .` desde la raíz de api-go, cmd/seed queda
// afuera). Genera ~1000 tickets demo repartidos en el tiempo para que los dashboards
// (ReportsPage, ExecutivePage, Grafana) tengan datos suficientes para verse bien.
// Uso: go run ./cmd/seed [-n 1000] [-db /data/todomark.db]
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"todomark/api/internal/store"

	_ "modernc.org/sqlite"
)

var (
	priorities = []string{"low", "medium", "high", "critical"}
	priorityW  = []int{35, 35, 20, 10} // pesos default, ver priorityWeightsFor() para variación por categoría
	channels   = []string{"web", "email", "glpi"}
	channelW   = []int{50, 30, 20}
	categories = []string{"mantenimiento", "plomeria", "electricidad", "limpieza", "seguridad", "administrativo", "otro"}
	// No uniforme: en un edificio real hay más tickets de mantenimiento/plomería que de seguridad/administrativo.
	categoryW   = []int{24, 20, 16, 14, 12, 9, 5}
	buildingIDs = []string{"bld_torre_norte", "bld_edificio_sur", "bld_res_central"}
	buildingW   = []int{40, 30, 30}
	agents      = []string{"Ana Torres", "Luis Fernández", "Marta Gómez", "Diego Ruiz", "Sofía Paredes"}
	slaTarget   = map[string]float64{"critical": 4, "high": 24, "medium": 72, "low": 168}

	// Prioridad correlacionada con categoría: electricidad/seguridad tienden a ser más urgentes,
	// limpieza/administrativo tienden a ser más leves — más realista que una mezcla pareja para todas.
	priorityWByCategory = map[string][]int{
		"electricidad":   {15, 30, 35, 20},
		"seguridad":      {10, 25, 35, 30},
		"mantenimiento":  {25, 35, 25, 15},
		"plomeria":       {25, 40, 25, 10},
		"limpieza":       {55, 35, 8, 2},
		"administrativo": {60, 30, 8, 2},
		"otro":           {40, 35, 18, 7},
	}

	titlesByCategory = map[string][]string{
		"plomeria":       {"Fuga de agua en baño", "Cañería rota", "Inodoro no descarga", "Filtración en el techo", "Presión de agua baja"},
		"electricidad":   {"Corte de luz en pasillo", "Tomacorriente no funciona", "Luces parpadeando", "Breaker se dispara", "Falla en el tablero eléctrico"},
		"mantenimiento":  {"Ascensor fuera de servicio", "Puerta de garage trabada", "Portón principal no cierra", "Bomba de agua con ruido", "Falla en el aire acondicionado"},
		"limpieza":       {"Área común sucia", "Basura acumulada en pasillo", "Falta limpieza en lobby", "Derrame sin limpiar en estacionamiento"},
		"seguridad":      {"Cámara de seguridad caída", "Puerta de emergencia sin llave", "Alarma de incendio con falla", "Acceso sin control en entrada"},
		"administrativo": {"Consulta sobre expensas", "Solicitud de certificado", "Reclamo por cobro duplicado", "Actualización de datos de contacto"},
		"otro":           {"Consulta general", "Solicitud de información", "Reporte de incidente varios"},
	}
)

func weightedPick(items []string, weights []int) string {
	total := 0
	for _, w := range weights {
		total += w
	}
	r := rand.Intn(total)
	for i, w := range weights {
		if r < w {
			return items[i]
		}
		r -= w
	}
	return items[len(items)-1]
}

func fmtTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func main() {
	n := flag.Int("n", 1000, "cantidad de tickets a generar")
	dbPath := flag.String("db", "/data/todomark.db", "ruta al archivo SQLite")
	days := flag.Int("days", 270, "ventana de días hacia atrás para repartir created_at")
	wipe := flag.Bool("wipe", false, "borrar todos los tickets existentes antes de sembrar (solo para re-seed de demo)")
	flag.Parse()

	if *wipe {
		raw, err := sql.Open("sqlite", *dbPath)
		if err != nil {
			log.Fatalf("no se pudo abrir la db para wipe: %v", err)
		}
		if _, err := raw.Exec("DELETE FROM tickets"); err != nil {
			log.Fatalf("wipe falló: %v", err)
		}
		raw.Close()
		fmt.Println("Wipe completo: tickets existentes borrados.")
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("no se pudo abrir el store: %v", err)
	}
	defer st.Close()

	now := time.Now().UTC()
	baseID := now.UnixNano()

	created := 0
	for i := 0; i < *n; i++ {
		// Tendencia de crecimiento: exponente >1 sesga hacia ageDays chico (más tickets recientes
		// que antiguos, como un negocio que va creciendo) en vez de un reparto plano en el tiempo.
		ageDays := int(rand.Float64() * rand.Float64() * float64(*days))
		createdAt := now.AddDate(0, 0, -ageDays).Add(-time.Duration(rand.Intn(24)) * time.Hour)

		category := weightedPick(categories, categoryW)
		priority := weightedPick(priorities, priorityWByCategory[category])
		channel := weightedPick(channels, channelW)

		// Tickets muy recientes (últimos 5 días) tienen más chance de seguir abiertos;
		// el resto, al ser "histórico", ya se resolvió (honesto: no todo queda abierto para siempre).
		var status string
		recent := ageDays < 5
		roll := rand.Intn(100)
		switch {
		case recent && roll < 40:
			status = "open"
		case recent && roll < 65:
			status = "in_progress"
		case roll < 5:
			status = "reopened"
		case roll < 20:
			status = "resolved"
		default:
			status = "closed"
		}

		var history []store.Event
		var closedAt *string
		updatedAt := createdAt

		addEvent := func(action, from, to string, at time.Time) {
			history = append(history, store.Event{Timestamp: fmtTime(at), User: agents[rand.Intn(len(agents))], Action: action, From: from, To: to})
			updatedAt = at
		}

		target := slaTarget[priority]
		// Multiplicador con cola larga: la mayoría cerca del objetivo, algunos bien pasados
		// (para que %SLA cumplido no salga 100% ni 0%, sea un número creíble).
		resolutionHours := target * (0.3 + rand.Float64()*2.2)
		resolvedAt := createdAt.Add(time.Duration(resolutionHours * float64(time.Hour)))
		if resolvedAt.After(now) {
			resolvedAt = now
		}
		progressAt := createdAt.Add(time.Duration(rand.Float64()*resolutionHours*0.3) * time.Hour)

		switch status {
		case "open":
			// sin eventos: recién creado
		case "in_progress":
			addEvent("status_change", "open", "in_progress", progressAt)
		case "resolved":
			addEvent("status_change", "open", "in_progress", progressAt)
			addEvent("status_change", "in_progress", "resolved", resolvedAt)
			c := fmtTime(resolvedAt)
			closedAt = &c
		case "closed":
			addEvent("status_change", "open", "in_progress", progressAt)
			addEvent("status_change", "in_progress", "resolved", resolvedAt)
			closedTime := resolvedAt.Add(time.Duration(rand.Intn(48)) * time.Hour)
			if closedTime.After(now) {
				closedTime = now
			}
			addEvent("status_change", "resolved", "closed", closedTime)
			c := fmtTime(closedTime)
			closedAt = &c
		case "reopened":
			addEvent("status_change", "open", "in_progress", progressAt)
			addEvent("status_change", "in_progress", "resolved", resolvedAt)
			reopenAt := resolvedAt.Add(time.Duration(1+rand.Intn(72)) * time.Hour)
			if reopenAt.After(now) {
				reopenAt = now
			}
			addEvent("status_change", "resolved", "reopened", reopenAt)
			// reopened queda activo: sin closed_at.
		}

		var buildingID *string
		var unitID *string
		isVIP := false
		if rand.Intn(100) < 55 {
			b := weightedPick(buildingIDs, buildingW)
			buildingID = &b
			isVIP = b == "bld_torre_norte"
			u := fmt.Sprintf("%d%c", 1+rand.Intn(12), 'A'+rune(rand.Intn(6)))
			unitID = &u
		}

		// CSAT levemente mejor en el edificio VIP (mismo criterio que un edificio con más
		// presupuesto/atención) — no es un número plano, aporta otra correlación entre paneles.
		var satisfaction *int
		if (status == "resolved" || status == "closed") && rand.Intn(100) < 60 {
			roll := rand.Intn(100)
			score := 5
			bump := 0
			if isVIP {
				bump = 10
			}
			switch {
			case roll < max(0, 5-bump):
				score = 1
			case roll < max(0, 15-bump):
				score = 2
			case roll < 30:
				score = 3
			case roll < max(0, 60-bump):
				score = 4
			default:
				score = 5
			}
			satisfaction = &score
		}

		var assignedTo *string
		if status != "open" && rand.Intn(100) < 85 {
			a := agents[rand.Intn(len(agents))]
			assignedTo = &a
		}

		email := fmt.Sprintf("vecino%d@demo.com", 1+rand.Intn(60))
		requester := fmt.Sprintf("Vecino Demo %d", 1+rand.Intn(60))
		titles := titlesByCategory[category]
		title := titles[rand.Intn(len(titles))]
		ch := channel
		cat := category

		t := &store.Ticket{
			ID:                fmt.Sprintf("%d", baseID-int64(i)),
			Title:             title,
			Description:       fmt.Sprintf("%s — reportado por %s.", title, requester),
			Status:            status,
			Priority:          priority,
			Requester:         requester,
			AssignedTo:        assignedTo,
			Email:             &email,
			ClosedAt:          closedAt,
			CreatedAt:         fmtTime(createdAt),
			UpdatedAt:         fmtTime(updatedAt),
			History:           history,
			Channel:           &ch,
			Category:          &cat,
			BuildingID:        buildingID,
			UnitID:            unitID,
			SatisfactionScore: satisfaction,
		}
		if t.History == nil {
			t.History = []store.Event{}
		}
		if err := st.Create(t); err != nil {
			log.Printf("ticket %d falló: %v", i, err)
			continue
		}
		created++
	}

	fmt.Printf("Seed completo: %d/%d tickets creados.\n", created, *n)
}
