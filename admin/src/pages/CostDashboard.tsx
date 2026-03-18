import { useEffect, useState, useRef, useCallback } from 'react';

interface CostEvent {
  event_type: string;
  workspace_id?: string;
  model?: string;
  provider?: string;
  tool_key?: string;
  input_tokens?: number;
  output_tokens?: number;
  cost_cents?: number;
  used_pct?: number;
  timestamp?: string;
}

interface DashboardData {
  current_hour_spend_cents: number;
  today_spend_cents: number;
  month_to_date_cents: number;
  projected_monthly_cents: number;
  top_5_workspaces: { workspace_id: string; workspace_name: string; spend_cents: number }[];
  top_5_models: { model: string; provider: string; spend_cents: number }[];
  cost_by_hour: { hour: string; spend_cents: number }[];
  cost_by_workflow_type: { workflow_type: string; spend_cents: number }[];
  cost_by_tool_key: { tool_key: string; spend_cents: number }[];
}

function formatCents(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

function formatHour(iso: string): string {
  const d = new Date(iso);
  const h = d.getHours();
  return h === 0 ? '12am' : h < 12 ? `${h}am` : h === 12 ? '12pm' : `${h - 12}pm`;
}

export default function CostDashboard() {
  const [dashboard, setDashboard] = useState<DashboardData | null>(null);
  const [liveEvents, setLiveEvents] = useState<CostEvent[]>([]);
  const [budgetAlert, setBudgetAlert] = useState<CostEvent | null>(null);
  const [sseStatus, setSSEStatus] = useState<'connected' | 'disconnected' | 'reconnecting'>('disconnected');
  const retryRef = useRef<number>(1);
  const eventSourceRef = useRef<EventSource | null>(null);

  // Fetch dashboard data.
  const fetchDashboard = useCallback(async () => {
    try {
      const resp = await fetch('/v1/admin/costs/dashboard', {
        headers: { 'X-User-Role': 'admin' },
      });
      if (resp.ok) {
        setDashboard(await resp.json());
      }
    } catch {
      // Silently retry on next interval.
    }
  }, []);

  // SSE connection with exponential backoff.
  const connectSSE = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    const es = new EventSource('/v1/admin/costs/live');
    eventSourceRef.current = es;

    es.onopen = () => {
      setSSEStatus('connected');
      retryRef.current = 1;
    };

    es.onmessage = (ev) => {
      try {
        const data: CostEvent = JSON.parse(ev.data);
        if (data.event_type === 'budget_alert' || data.event_type === 'budget_critical') {
          setBudgetAlert(data);
        }
        setLiveEvents((prev) => [data, ...prev].slice(0, 10));
      } catch {
        // Ignore malformed events.
      }
    };

    es.onerror = () => {
      es.close();
      setSSEStatus('reconnecting');
      const delay = Math.min(retryRef.current * 1000, 30000);
      retryRef.current *= 2;
      setTimeout(connectSSE, delay);
    };
  }, []);

  useEffect(() => {
    fetchDashboard();
    connectSSE();
    const interval = setInterval(fetchDashboard, 60000);
    return () => {
      clearInterval(interval);
      eventSourceRef.current?.close();
    };
  }, [fetchDashboard, connectSSE]);

  const handleExport = () => {
    const now = new Date();
    const start = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-01`;
    const end = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
    window.open(`/v1/admin/costs/export?format=csv&start=${start}&end=${end}`, '_blank');
  };

  return (
    <div style={{ padding: '24px', fontFamily: 'system-ui, sans-serif', maxWidth: '1200px', margin: '0 auto' }}>
      <h1 style={{ fontSize: '24px', fontWeight: 600, marginBottom: '16px' }}>LLM Cost Dashboard</h1>

      {/* Budget Alert Banner */}
      {budgetAlert && (
        <div style={{
          background: '#fee2e2', border: '1px solid #ef4444', borderRadius: '8px',
          padding: '12px 16px', marginBottom: '16px', color: '#b91c1c',
        }}>
          <strong>Budget Alert:</strong> Workspace {budgetAlert.workspace_id} at{' '}
          {budgetAlert.used_pct ? `${(budgetAlert.used_pct * 100).toFixed(0)}%` : 'critical level'}
          <button onClick={() => setBudgetAlert(null)} style={{ float: 'right', cursor: 'pointer', border: 'none', background: 'none', color: '#b91c1c' }}>
            Dismiss
          </button>
        </div>
      )}

      {/* SSE Status */}
      <div style={{ marginBottom: '16px', fontSize: '12px', color: '#6b7280' }}>
        Live feed: {sseStatus === 'connected' ? '🟢 Connected' : sseStatus === 'reconnecting' ? '🟡 Reconnecting...' : '🔴 Disconnected'}
      </div>

      {/* Summary Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '16px', marginBottom: '24px' }}>
        {[
          { label: 'Current Hour', value: dashboard?.current_hour_spend_cents },
          { label: 'Today', value: dashboard?.today_spend_cents },
          { label: 'Month-to-Date', value: dashboard?.month_to_date_cents },
          { label: 'Projected Monthly', value: dashboard?.projected_monthly_cents },
        ].map((card) => (
          <div key={card.label} style={{
            background: '#fff', border: '1px solid #e5e7eb', borderRadius: '8px',
            padding: '16px', textAlign: 'center',
          }}>
            <div style={{ fontSize: '12px', color: '#6b7280', marginBottom: '4px' }}>{card.label}</div>
            <div style={{ fontSize: '24px', fontWeight: 700 }}>
              {card.value != null ? formatCents(card.value) : '—'}
            </div>
          </div>
        ))}
      </div>

      {/* Hourly Chart (simple bar representation) */}
      {dashboard?.cost_by_hour && dashboard.cost_by_hour.length > 0 && (
        <div style={{ marginBottom: '24px' }}>
          <h2 style={{ fontSize: '18px', fontWeight: 600, marginBottom: '12px' }}>24-Hour Cost Trend</h2>
          <div style={{ display: 'flex', alignItems: 'flex-end', height: '120px', gap: '2px' }}>
            {dashboard.cost_by_hour.map((h, i) => {
              const max = Math.max(...dashboard.cost_by_hour.map((x) => x.spend_cents), 1);
              const height = (h.spend_cents / max) * 100;
              return (
                <div key={i} style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                  <div style={{
                    width: '100%', background: '#3b82f6', borderRadius: '2px 2px 0 0',
                    height: `${height}%`, minHeight: '2px',
                  }} title={`${formatHour(h.hour)}: ${formatCents(h.spend_cents)}`} />
                  <div style={{ fontSize: '8px', color: '#9ca3af', marginTop: '2px' }}>{formatHour(h.hour)}</div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Top Workspaces and Models side by side */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '24px', marginBottom: '24px' }}>
        <div>
          <h2 style={{ fontSize: '18px', fontWeight: 600, marginBottom: '12px' }}>Top Workspaces</h2>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead><tr style={{ borderBottom: '1px solid #e5e7eb' }}>
              <th style={{ textAlign: 'left', padding: '8px' }}>Workspace</th>
              <th style={{ textAlign: 'right', padding: '8px' }}>Spend</th>
            </tr></thead>
            <tbody>
              {(dashboard?.top_5_workspaces || []).map((ws) => (
                <tr key={ws.workspace_id} style={{ borderBottom: '1px solid #f3f4f6' }}>
                  <td style={{ padding: '8px', fontSize: '14px' }}>{ws.workspace_name}</td>
                  <td style={{ padding: '8px', textAlign: 'right', fontWeight: 600 }}>{formatCents(ws.spend_cents)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div>
          <h2 style={{ fontSize: '18px', fontWeight: 600, marginBottom: '12px' }}>Top Models</h2>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead><tr style={{ borderBottom: '1px solid #e5e7eb' }}>
              <th style={{ textAlign: 'left', padding: '8px' }}>Model</th>
              <th style={{ textAlign: 'left', padding: '8px' }}>Provider</th>
              <th style={{ textAlign: 'right', padding: '8px' }}>Spend</th>
            </tr></thead>
            <tbody>
              {(dashboard?.top_5_models || []).map((m, i) => (
                <tr key={i} style={{ borderBottom: '1px solid #f3f4f6' }}>
                  <td style={{ padding: '8px', fontSize: '14px' }}>{m.model}</td>
                  <td style={{ padding: '8px', fontSize: '14px' }}>{m.provider}</td>
                  <td style={{ padding: '8px', textAlign: 'right', fontWeight: 600 }}>{formatCents(m.spend_cents)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Live Ticker */}
      <div style={{ marginBottom: '24px' }}>
        <h2 style={{ fontSize: '18px', fontWeight: 600, marginBottom: '12px' }}>Live Events</h2>
        <div style={{ background: '#f9fafb', border: '1px solid #e5e7eb', borderRadius: '8px', padding: '12px', maxHeight: '200px', overflow: 'auto' }}>
          {liveEvents.length === 0 ? (
            <div style={{ color: '#9ca3af', textAlign: 'center' }}>Waiting for events...</div>
          ) : (
            liveEvents.map((ev, i) => (
              <div key={i} style={{ padding: '4px 0', borderBottom: '1px solid #f3f4f6', fontSize: '13px', fontFamily: 'monospace' }}>
                <span style={{ color: '#6b7280' }}>{ev.timestamp || new Date().toISOString()}</span>{' '}
                <span style={{ color: ev.event_type?.includes('alert') ? '#ef4444' : '#3b82f6' }}>{ev.event_type}</span>{' '}
                {ev.model && <span>{ev.model}</span>}{' '}
                {ev.cost_cents != null && <span style={{ fontWeight: 600 }}>{formatCents(ev.cost_cents)}</span>}
              </div>
            ))
          )}
        </div>
      </div>

      {/* Export Button */}
      <button
        onClick={handleExport}
        style={{
          background: '#3b82f6', color: '#fff', border: 'none', borderRadius: '6px',
          padding: '10px 20px', cursor: 'pointer', fontSize: '14px', fontWeight: 500,
        }}
      >
        Export CSV (Current Month)
      </button>
    </div>
  );
}
