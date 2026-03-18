package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// CostStreamPool is the database pool used by cost endpoints.
var CostStreamPool *pgxpool.Pool

// CostRedisClient is the Redis client for SSE pub/sub.
var CostRedisClient *goredis.Client

// CostSSEChannel is the Redis pub/sub channel for live cost events.
const CostSSEChannel = "llm:cost:events"

// RegisterCostRoutes adds cost dashboard routes to the mux.
func RegisterCostRoutes(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /v1/admin/costs/live", adminOnly(handleCostLiveStream(svc)))
	mux.HandleFunc("GET /v1/admin/costs/dashboard", adminOnly(handleCostDashboard(svc)))
	mux.HandleFunc("GET /v1/admin/costs/export", adminOnly(handleCostExport(svc)))
}

// handleCostLiveStream serves an SSE stream of real-time LLM cost events.
func handleCostLiveStream(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if CostRedisClient == nil {
			fmt.Fprintf(w, "data: {\"error\": \"redis not available\"}\n\n")
			flusher.Flush()
			return
		}

		ctx := r.Context()
		sub := CostRedisClient.Subscribe(ctx, CostSSEChannel)
		defer sub.Close()

		ch := sub.Channel()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", msg.Payload)
				flusher.Flush()
			case <-ticker.C:
				fmt.Fprintf(w, ": ping\n\n")
				flusher.Flush()
			}
		}
	}
}

// handleCostDashboard returns aggregated cost dashboard data.
func handleCostDashboard(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := CostStreamPool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		ctx := r.Context()
		provider := r.URL.Query().Get("provider")
		workspaceID := r.URL.Query().Get("workspace_id")
		workflowType := r.URL.Query().Get("workflow_type")

		// Build filter clause.
		filterClause, filterArgs := buildFilterClause(provider, workspaceID, workflowType, 1)

		result := map[string]interface{}{}

		// Current hour spend.
		var currentHour float64
		_ = pool.QueryRow(ctx,
			`SELECT COALESCE(SUM(cost_cents), 0) FROM llm_invocations
			 WHERE created_at >= date_trunc('hour', NOW())`+filterClause,
			filterArgs...,
		).Scan(&currentHour)
		result["current_hour_spend_cents"] = currentHour

		// Today spend.
		var todaySpend float64
		_ = pool.QueryRow(ctx,
			`SELECT COALESCE(SUM(cost_cents), 0) FROM llm_invocations
			 WHERE created_at >= date_trunc('day', NOW())`+filterClause,
			filterArgs...,
		).Scan(&todaySpend)
		result["today_spend_cents"] = todaySpend

		// Month-to-date.
		var mtdSpend float64
		_ = pool.QueryRow(ctx,
			`SELECT COALESCE(SUM(cost_cents), 0) FROM llm_invocations
			 WHERE created_at >= date_trunc('month', NOW())`+filterClause,
			filterArgs...,
		).Scan(&mtdSpend)
		result["month_to_date_cents"] = mtdSpend

		// Projected monthly.
		now := time.Now()
		daysElapsed := float64(now.Day())
		daysInMonth := float64(daysInCurrentMonth())
		projected := 0.0
		if daysElapsed > 0 {
			projected = mtdSpend / daysElapsed * daysInMonth
		}
		result["projected_monthly_cents"] = projected

		// Top 5 workspaces.
		wsRows, _ := pool.Query(ctx,
			`SELECT li.workspace_id, COALESCE(w.name, li.workspace_id::text) as name, SUM(li.cost_cents) as spend
			 FROM llm_invocations li
			 LEFT JOIN workspaces w ON w.id = li.workspace_id
			 WHERE li.created_at >= date_trunc('month', NOW())`+filterClause+`
			 GROUP BY li.workspace_id, w.name
			 ORDER BY spend DESC LIMIT 5`,
			filterArgs...,
		)
		var topWorkspaces []map[string]interface{}
		if wsRows != nil {
			defer wsRows.Close()
			for wsRows.Next() {
				var wsID, name string
				var spend float64
				if err := wsRows.Scan(&wsID, &name, &spend); err == nil {
					topWorkspaces = append(topWorkspaces, map[string]interface{}{
						"workspace_id": wsID, "workspace_name": name, "spend_cents": spend,
					})
				}
			}
		}
		result["top_5_workspaces"] = topWorkspaces

		// Top 5 models.
		modelRows, _ := pool.Query(ctx,
			`SELECT model, provider, SUM(cost_cents) as spend
			 FROM llm_invocations
			 WHERE created_at >= date_trunc('month', NOW())`+filterClause+`
			 GROUP BY model, provider
			 ORDER BY spend DESC LIMIT 5`,
			filterArgs...,
		)
		var topModels []map[string]interface{}
		if modelRows != nil {
			defer modelRows.Close()
			for modelRows.Next() {
				var model, prov string
				var spend float64
				if err := modelRows.Scan(&model, &prov, &spend); err == nil {
					topModels = append(topModels, map[string]interface{}{
						"model": model, "provider": prov, "spend_cents": spend,
					})
				}
			}
		}
		result["top_5_models"] = topModels

		// 24h hourly histogram.
		hourRows, _ := pool.Query(ctx,
			`SELECT date_trunc('hour', created_at) as hour, SUM(cost_cents) as spend
			 FROM llm_invocations
			 WHERE created_at >= NOW() - INTERVAL '24 hours'`+filterClause+`
			 GROUP BY hour ORDER BY hour`,
			filterArgs...,
		)
		var costByHour []map[string]interface{}
		if hourRows != nil {
			defer hourRows.Close()
			for hourRows.Next() {
				var hour time.Time
				var spend float64
				if err := hourRows.Scan(&hour, &spend); err == nil {
					costByHour = append(costByHour, map[string]interface{}{
						"hour": hour, "spend_cents": spend,
					})
				}
			}
		}
		result["cost_by_hour"] = costByHour

		// Workflow type breakdown.
		wfRows, _ := pool.Query(ctx,
			`SELECT COALESCE(workflow_type, 'unknown') as wf, SUM(cost_cents) as spend
			 FROM llm_invocations
			 WHERE created_at >= date_trunc('month', NOW())`+filterClause+`
			 GROUP BY wf ORDER BY spend DESC`,
			filterArgs...,
		)
		var costByWF []map[string]interface{}
		if wfRows != nil {
			defer wfRows.Close()
			for wfRows.Next() {
				var wf string
				var spend float64
				if err := wfRows.Scan(&wf, &spend); err == nil {
					costByWF = append(costByWF, map[string]interface{}{
						"workflow_type": wf, "spend_cents": spend,
					})
				}
			}
		}
		result["cost_by_workflow_type"] = costByWF

		// Tool key breakdown (top 10).
		toolRows, _ := pool.Query(ctx,
			`SELECT COALESCE(tool_key, 'direct') as tk, SUM(cost_cents) as spend
			 FROM llm_invocations
			 WHERE created_at >= date_trunc('month', NOW())`+filterClause+`
			 GROUP BY tk ORDER BY spend DESC LIMIT 10`,
			filterArgs...,
		)
		var costByTool []map[string]interface{}
		if toolRows != nil {
			defer toolRows.Close()
			for toolRows.Next() {
				var tk string
				var spend float64
				if err := toolRows.Scan(&tk, &spend); err == nil {
					costByTool = append(costByTool, map[string]interface{}{
						"tool_key": tk, "spend_cents": spend,
					})
				}
			}
		}
		result["cost_by_tool_key"] = costByTool

		writeJSON(w, http.StatusOK, result)
	}
}

// handleCostExport exports cost data as CSV.
func handleCostExport(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := CostStreamPool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		startStr := r.URL.Query().Get("start")
		endStr := r.URL.Query().Get("end")
		if startStr == "" || endStr == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "start and end date required"})
			return
		}

		start, err := time.Parse("2006-01-02", startStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start date"})
			return
		}
		end, err := time.Parse("2006-01-02", endStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end date"})
			return
		}

		provider := r.URL.Query().Get("provider")
		workspaceID := r.URL.Query().Get("workspace_id")
		workflowType := r.URL.Query().Get("workflow_type")

		filterClause, filterArgs := buildFilterClause(provider, workspaceID, workflowType, 3)
		allArgs := append([]any{start, end.Add(24 * time.Hour)}, filterArgs...)

		rows, qErr := pool.Query(r.Context(),
			`SELECT li.created_at, li.workspace_id, COALESCE(w.name, ''), li.provider, li.model,
			        COALESCE(li.tool_key, ''), li.input_tokens, li.output_tokens, li.cost_cents
			 FROM llm_invocations li
			 LEFT JOIN workspaces w ON w.id = li.workspace_id
			 WHERE li.created_at >= $1 AND li.created_at < $2`+filterClause+`
			 ORDER BY li.created_at`,
			allArgs...,
		)
		if qErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "query failed"})
			return
		}
		defer rows.Close()

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition",
			fmt.Sprintf(`attachment; filename="costs_%s_%s.csv"`, startStr, endStr))

		fmt.Fprintln(w, "timestamp,workspace_id,workspace_name,provider,model,tool_key,input_tokens,output_tokens,cost_cents")
		for rows.Next() {
			var ts time.Time
			var wsID, wsName, prov, model, toolKey string
			var inputTok, outputTok int
			var cost float64
			if err := rows.Scan(&ts, &wsID, &wsName, &prov, &model, &toolKey, &inputTok, &outputTok, &cost); err != nil {
				continue
			}
			fmt.Fprintf(w, "%s,%s,%s,%s,%s,%s,%d,%d,%.2f\n",
				ts.Format(time.RFC3339), wsID, wsName, prov, model, toolKey, inputTok, outputTok, cost)
		}
	}
}

func buildFilterClause(provider, workspaceID, workflowType string, startArgIdx int) (string, []any) {
	clause := ""
	args := []any{}
	idx := startArgIdx

	if provider != "" {
		clause += fmt.Sprintf(" AND provider = $%d", idx)
		args = append(args, provider)
		idx++
	}
	if workspaceID != "" {
		clause += fmt.Sprintf(" AND workspace_id = $%d", idx)
		args = append(args, workspaceID)
		idx++
	}
	if workflowType != "" {
		clause += fmt.Sprintf(" AND workflow_type = $%d", idx)
		args = append(args, workflowType)
		idx++
	}
	return clause, args
}

func daysInCurrentMonth() int {
	now := time.Now()
	return time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// PublishCostEvent publishes a cost event to the Redis SSE channel.
func PublishCostEvent(ctx context.Context, redis *goredis.Client, event map[string]interface{}, logger *slog.Logger) {
	if redis == nil {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		logger.Warn("cost_event_marshal_error", "error", err)
		return
	}
	if pubErr := redis.Publish(ctx, CostSSEChannel, string(data)).Err(); pubErr != nil {
		logger.Warn("cost_event_publish_error", "error", pubErr)
	}
}
