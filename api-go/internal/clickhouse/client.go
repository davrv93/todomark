// Package clickhouse sincroniza el store SQLite operacional hacia ClickHouse
// para la capa analítica (PLAN.md §19). Sigue el mismo patrón que
// internal/glpi: Client + NewFromEnv() + Configured(), y un Syncer separado
// (sync.go) que corre en background.
package clickhouse

import (
	"context"
	_ "embed"
	"os"
	"strings"
	"sync"

	"github.com/ClickHouse/clickhouse-go/v2"
)

//go:embed schema.sql
var schemaSQL string

// Client mantiene la configuración y la conexión (perezosa) hacia ClickHouse.
type Client struct {
	addr     string
	db       string
	user     string
	password string

	mu   sync.Mutex
	conn clickhouse.Conn
}

// NewFromEnv lee CLICKHOUSE_URL (default clickhouse:9000), CLICKHOUSE_DB
// (default todomark), CLICKHOUSE_USER y CLICKHOUSE_PASSWORD (default
// todomark/todomark, igual que las credenciales dev de postgres/evolution en
// docker-compose.yml) — igual que glpi.NewFromEnv lee sus GLPI_*. La imagen
// oficial de ClickHouse deshabilita el acceso por red del usuario 'default'
// si no se define CLICKHOUSE_USER/CLICKHOUSE_PASSWORD, así que hace falta un
// usuario explícito para conectar desde otro contenedor.
func NewFromEnv() *Client {
	return &Client{
		addr:     envOr("CLICKHOUSE_URL", "clickhouse:9000"),
		db:       envOr("CLICKHOUSE_DB", "todomark"),
		user:     envOr("CLICKHOUSE_USER", "todomark"),
		password: envOr("CLICKHOUSE_PASSWORD", "todomark"),
	}
}

// Configured indica si hay suficiente configuración para intentar conectar.
func (c *Client) Configured() bool {
	return c.addr != "" && c.db != ""
}

// conn abre la conexión (si hace falta) y asegura el esquema una sola vez.
func (c *Client) getConn(ctx context.Context) (clickhouse.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn, nil
	}
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{c.addr},
		Auth: clickhouse.Auth{
			Database: c.db,
			Username: c.user,
			Password: c.password,
		},
	})
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(ctx); err != nil {
		return nil, err
	}
	if err := ensureSchema(ctx, conn); err != nil {
		return nil, err
	}
	c.conn = conn
	return conn, nil
}

// ensureSchema ejecuta schema.sql (CREATE TABLE IF NOT EXISTS ...) sentencia
// por sentencia; el driver de ClickHouse no soporta varias sentencias por Exec.
func ensureSchema(ctx context.Context, conn clickhouse.Conn) error {
	for _, stmt := range splitStatements(schemaSQL) {
		if err := conn.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// splitStatements quita las líneas de comentario ("-- ...", que pueden traer
// su propio ";" en el texto) antes de partir por ";" — partir primero y
// filtrar comentarios después rompe si un comentario contiene un ";".
func splitStatements(sqlScript string) []string {
	var withoutComments []string
	for _, line := range strings.Split(sqlScript, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		withoutComments = append(withoutComments, line)
	}
	var out []string
	for _, stmt := range strings.Split(strings.Join(withoutComments, "\n"), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			out = append(out, stmt)
		}
	}
	return out
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
