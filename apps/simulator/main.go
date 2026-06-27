package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Machine struct {
	ID       string
	Name     string
	Type     string
	Location string
	Status   string
	APIKey   string
	OrgID    string
}

type MetricDef struct {
	Name string
	Min  float64
	Max  float64
}

type MetricProfile struct {
	Metrics    []MetricDef
	ErrorCodes []string
}

var profiles = map[string]MetricProfile{
	"cnc": {
		Metrics: []MetricDef{
			{Name: "temperature_celsius", Min: 35, Max: 85},
			{Name: "vibration_mm_s", Min: 0.5, Max: 8},
			{Name: "rpm", Min: 500, Max: 12000},
			{Name: "power_kw", Min: 2, Max: 50},
		},
		ErrorCodes: []string{"E-203", "E-401", "E-302"},
	},
	"pump": {
		Metrics: []MetricDef{
			{Name: "temperature_celsius", Min: 20, Max: 60},
			{Name: "vibration_mm_s", Min: 1, Max: 12},
			{Name: "pressure_bar", Min: 1, Max: 10},
			{Name: "flow_l_min", Min: 10, Max: 500},
		},
		ErrorCodes: []string{"E-105", "E-203", "E-401"},
	},
	"compressor": {
		Metrics: []MetricDef{
			{Name: "temperature_celsius", Min: 30, Max: 90},
			{Name: "vibration_mm_s", Min: 2, Max: 15},
			{Name: "pressure_bar", Min: 4, Max: 15},
			{Name: "rpm", Min: 1000, Max: 3000},
		},
		ErrorCodes: []string{"E-203", "E-401", "E-105"},
	},
	"conveyor": {
		Metrics: []MetricDef{
			{Name: "temperature_celsius", Min: 25, Max: 45},
			{Name: "vibration_mm_s", Min: 0.5, Max: 5},
			{Name: "speed_m_s", Min: 0.5, Max: 5},
			{Name: "load_percent", Min: 0, Max: 100},
		},
		ErrorCodes: []string{"E-401", "E-302"},
	},
	"generator": {
		Metrics: []MetricDef{
			{Name: "temperature_celsius", Min: 40, Max: 120},
			{Name: "vibration_mm_s", Min: 1, Max: 10},
			{Name: "rpm", Min: 1500, Max: 3600},
			{Name: "output_kw", Min: 10, Max: 500},
		},
		ErrorCodes: []string{"E-203", "E-302", "E-401"},
	},
	"robot_arm": {
		Metrics: []MetricDef{
			{Name: "temperature_celsius", Min: 30, Max: 70},
			{Name: "vibration_mm_s", Min: 0.5, Max: 6},
			{Name: "joint_temp_celsius", Min: 25, Max: 65},
			{Name: "power_kw", Min: 1, Max: 15},
		},
		ErrorCodes: []string{"E-203", "E-401"},
	},
}

var defaultProfile = profiles["cnc"]

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	databaseURL := mustEnv("DATABASE_URL")
	apiURL := strings.TrimRight(mustEnv("API_URL"), "/")
	interval := getEnvDuration("INTERVAL", 15*time.Second)
	errorProb := getEnvFloat("ERROR_PROBABILITY", 0.08)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		slog.Error("connect to postgres", slog.String("err", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	slog.Info("starting machine telemetry simulator",
		slog.Duration("interval", interval),
		slog.Float64("error_probability", errorProb),
		slog.String("api_url", apiURL),
	)

	if err := provisionAPIKeys(ctx, pool); err != nil {
		slog.Warn("provision api keys", slog.String("err", err.Error()))
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runCycle(ctx, pool, apiURL, errorProb)

	for range ticker.C {
		runCycle(ctx, pool, apiURL, errorProb)
	}
}

func provisionAPIKeys(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
		SELECT id FROM machines
		WHERE metadata->>'api_key' IS NULL OR metadata->>'api_key' = ''
	`)
	if err != nil {
		return fmt.Errorf("query machines without api key: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(ids) == 0 {
		slog.Info("all machines already have api keys")
		return nil
	}

	for _, id := range ids {
		key := generateAPIKey()
		if _, err := pool.Exec(ctx, `
			UPDATE machines
			SET metadata = jsonb_set(COALESCE(metadata, '{}'::jsonb), '{api_key}', to_jsonb($2::text)),
			    updated_at = now()
			WHERE id = $1
		`, id, key); err != nil {
			slog.Error("set api key", slog.String("machine_id", id), slog.String("err", err.Error()))
			continue
		}
		slog.Info("provisioned api key", slog.String("machine_id", id))
	}
	return nil
}

func runCycle(ctx context.Context, pool *pgxpool.Pool, apiURL string, errorProb float64) {
	machines, err := fetchMachines(ctx, pool)
	if err != nil {
		slog.Error("fetch machines", slog.String("err", err.Error()))
		return
	}

	if len(machines) == 0 {
		slog.Info("no machines in db, skipping cycle")
		return
	}

	var wg sync.WaitGroup
	for i := range machines {
		wg.Add(1)
		m := machines[i]
		go func() {
			defer wg.Done()
			sendTelemetry(ctx, apiURL, m, errorProb)
		}()
	}
	wg.Wait()

	slog.Debug("cycle complete", slog.Int("machines", len(machines)))
}

func fetchMachines(ctx context.Context, pool *pgxpool.Pool) ([]Machine, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, name, machine_type, location, status,
		       COALESCE(metadata->>'api_key', '') AS api_key,
		       org_id
		FROM machines
		WHERE status IN ('operational', 'degraded')
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var machines []Machine
	for rows.Next() {
		var m Machine
		if err := rows.Scan(&m.ID, &m.Name, &m.Type, &m.Location, &m.Status, &m.APIKey, &m.OrgID); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		machines = append(machines, m)
	}
	return machines, rows.Err()
}

func sendTelemetry(ctx context.Context, apiURL string, m Machine, errorProb float64) {
	if m.APIKey == "" {
		slog.Warn("skipping machine without api key",
			slog.String("machine_id", m.ID),
			slog.String("machine_name", m.Name),
		)
		return
	}

	profile, ok := profiles[m.Type]
	if !ok {
		profile = defaultProfile
	}

	metrics := make(map[string]any)
	for _, md := range profile.Metrics {
		val := randFloat(md.Min, md.Max)

		// If the machine is degraded, bias metrics toward the upper 30%
		if m.Status == "degraded" {
			threshold := md.Min + (md.Max-md.Min)*0.7
			if val < threshold {
				val = threshold + randFloat(0, md.Max-threshold)
			}
		}

		metrics[md.Name] = round(val, 2)
	}

	body := map[string]any{
		"metrics":    metrics,
		"source":     "simulator",
		"error_code": "",
	}

	if randFloat(0, 1) < errorProb && len(profile.ErrorCodes) > 0 {
		ec := profile.ErrorCodes[randInt(len(profile.ErrorCodes))]
		body["error_code"] = ec
		slog.Info("injecting error",
			slog.String("machine_id", m.ID),
			slog.String("machine_name", m.Name),
			slog.String("error_code", ec),
		)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		slog.Error("marshal telemetry", slog.String("machine_id", m.ID), slog.String("err", err.Error()))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		apiURL+"/machines/"+m.ID+"/telemetry",
		bytes.NewReader(payload),
	)
	if err != nil {
		slog.Error("create request", slog.String("machine_id", m.ID), slog.String("err", err.Error()))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("send telemetry",
			slog.String("machine_id", m.ID),
			slog.String("err", err.Error()),
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Warn("telemetry rejected",
			slog.String("machine_id", m.ID),
			slog.Int("status", resp.StatusCode),
		)
		return
	}

	slog.Debug("telemetry sent",
		slog.String("machine_id", m.ID),
		slog.String("machine_name", m.Name),
	)
}

// ── Helpers ──────────────────────────────────────────────────────────────

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var not set", slog.String("key", key))
		os.Exit(1)
	}
	return v
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func getEnvFloat(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var f float64
	if _, err := fmt.Sscanf(v, "%f", &f); err != nil {
		return def
	}
	return f
}

func generateAPIKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return "sim_" + hex.EncodeToString(b)
}

func randFloat(min, max float64) float64 {
	n, err := rand.Int(rand.Reader, big.NewInt(1<<52))
	if err != nil {
		panic(err)
	}
	return min + float64(n.Int64())/float64(1<<52)*(max-min)
}

func randInt(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		panic(err)
	}
	return int(n.Int64())
}

func round(v float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
}
