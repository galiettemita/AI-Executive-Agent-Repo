import { useEffect, useState, useCallback } from 'react';
import {
  fetchCapabilityRecommendations,
  ingestTestMessage,
  type ServiceHealth,
  type CapabilityRecommendation,
} from '../api/client';

interface ServiceCard { name: string; port: number; healthUrl: string; }

const SERVICES: ServiceCard[] = [
  { name: 'Gateway',         port: 18080, healthUrl: 'http://localhost:18080/health/deep' },
  { name: 'Brain',           port: 18081, healthUrl: 'http://localhost:18081/health/deep' },
  { name: 'Control',         port: 18082, healthUrl: 'http://localhost:18082/health/deep' },
  { name: 'Executor',        port: 18083, healthUrl: 'http://localhost:18083/health/deep' },
  { name: 'Temporal Worker', port: 18084, healthUrl: 'http://localhost:18084/health/deep' },
  { name: 'Canvas',          port: 18793, healthUrl: 'http://localhost:18793/health/deep' },
];

type HealthStatus = 'loading' | 'healthy' | 'degraded' | 'down';
interface CardState { status: HealthStatus; version?: string; uptime?: string; error?: string; }

function uptimeLabel(ms: number): string {
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`;
  return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`;
}

function StatusDot({ status }: { status: HealthStatus }) {
  const c: Record<HealthStatus, string> = { loading: '#999', healthy: '#22c55e', degraded: '#f59e0b', down: '#ef4444' };
  return <span style={{ display: 'inline-block', width: 10, height: 10, borderRadius: '50%', backgroundColor: c[status], marginRight: 6 }} />;
}

export default function Dashboard() {
  const [health, setHealth] = useState<Record<string, CardState>>(
    Object.fromEntries(SERVICES.map(s => [s.name, { status: 'loading' as HealthStatus }]))
  );
  const [recs, setRecs] = useState<CapabilityRecommendation[]>([]);
  const [testMsg, setTestMsg] = useState('Book me a meeting tomorrow at 10am');
  const [testWs, setTestWs] = useState('ws-admin-test');
  const [testResult, setTestResult] = useState<string | null>(null);
  const [sending, setSending] = useState(false);

  const fetchAll = useCallback(async () => {
    for (const svc of SERVICES) {
      try {
        const r = await fetch(svc.healthUrl);
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        const d: ServiceHealth = await r.json();
        setHealth(p => ({ ...p, [svc.name]: { status: d.status === 'healthy' ? 'healthy' : 'degraded', version: d.version, uptime: d.uptime_ms ? uptimeLabel(d.uptime_ms) : undefined } }));
      } catch (e: any) {
        setHealth(p => ({ ...p, [svc.name]: { status: 'down', error: e.message } }));
      }
    }
  }, []);

  useEffect(() => { fetchAll(); fetchCapabilityRecommendations(testWs).then(r => setRecs(Array.isArray(r) ? r : [])).catch(() => setRecs([])); const i = setInterval(fetchAll, 30000); return () => clearInterval(i); }, [fetchAll, testWs]);

  const send = async () => {
    setSending(true); setTestResult(null);
    try { const r = await ingestTestMessage(testWs, testMsg); setTestResult(`✓ ${r.status} — ${r.workflow_id || r.message_id}`); }
    catch (e: any) { setTestResult(`✗ ${e.message}`); }
    finally { setSending(false); }
  };

  const ok = Object.values(health).filter(s => s.status === 'healthy').length;

  return (
    <main style={{ fontFamily: 'Inter,system-ui,sans-serif', padding: '32px 40px', background: '#f9fafb', minHeight: '100vh' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 32 }}>
        <div>
          <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700 }}>Brevio Admin Dashboard</h1>
          <p style={{ margin: '4px 0 0', color: '#6b7280', fontSize: 13 }}>{ok}/{SERVICES.length} services healthy · auto-refreshes every 30s</p>
        </div>
        <button onClick={fetchAll} style={{ padding: '8px 16px', background: '#2563eb', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontSize: 13 }}>Refresh</button>
      </div>

      <section style={{ marginBottom: 32 }}>
        <h2 style={{ fontSize: 15, fontWeight: 600, color: '#374151', marginBottom: 12 }}>Service Health</h2>
        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
          {SERVICES.map(s => (
            <div key={s.name} style={{ background: '#fff', border: '1px solid #e5e7eb', borderRadius: 8, padding: '16px 20px', minWidth: 180 }}>
              <div style={{ display: 'flex', alignItems: 'center', marginBottom: 8 }}><StatusDot status={health[s.name]?.status ?? 'loading'} /><strong style={{ fontSize: 14 }}>{s.name}</strong></div>
              <div style={{ fontSize: 12, color: '#6b7280' }}>Port {s.port}</div>
              {health[s.name]?.version && <div style={{ fontSize: 12, color: '#6b7280', marginTop: 4 }}>v{health[s.name].version}</div>}
              {health[s.name]?.uptime && <div style={{ fontSize: 12, color: '#6b7280' }}>Up {health[s.name].uptime}</div>}
              {health[s.name]?.error && <div style={{ fontSize: 11, color: '#ef4444', marginTop: 4 }}>{health[s.name].error}</div>}
            </div>
          ))}
        </div>
      </section>

      <section style={{ marginBottom: 32 }}>
        <h2 style={{ fontSize: 15, fontWeight: 600, color: '#374151', marginBottom: 12 }}>Capability Recommendations — {testWs}</h2>
        {recs.length === 0 ? <p style={{ color: '#9ca3af', fontSize: 13 }}>No recommendations yet.</p> : (
          <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>{recs.map(r => (
            <li key={r.id} style={{ background: '#fff', border: '1px solid #e5e7eb', borderRadius: 6, padding: '10px 14px', marginBottom: 8, fontSize: 13 }}>
              <strong>{r.capability_key}</strong><span style={{ color: '#6b7280', marginLeft: 8 }}>{(r.confidence * 100).toFixed(0)}%</span>
              <div style={{ color: '#9ca3af', marginTop: 2 }}>{r.reason}</div>
            </li>
          ))}</ul>
        )}
      </section>

      <section>
        <h2 style={{ fontSize: 15, fontWeight: 600, color: '#374151', marginBottom: 12 }}>Send Test Message</h2>
        <div style={{ background: '#fff', border: '1px solid #e5e7eb', borderRadius: 8, padding: '20px 24px', maxWidth: 600 }}>
          <div style={{ marginBottom: 12 }}>
            <label style={{ fontSize: 12, fontWeight: 600, display: 'block', marginBottom: 4 }}>Workspace ID</label>
            <input value={testWs} onChange={e => setTestWs(e.target.value)} style={{ width: '100%', padding: '8px 10px', border: '1px solid #d1d5db', borderRadius: 5, fontSize: 13, boxSizing: 'border-box' as const }} />
          </div>
          <div style={{ marginBottom: 16 }}>
            <label style={{ fontSize: 12, fontWeight: 600, display: 'block', marginBottom: 4 }}>Message Content</label>
            <textarea value={testMsg} onChange={e => setTestMsg(e.target.value)} rows={3} style={{ width: '100%', padding: '8px 10px', border: '1px solid #d1d5db', borderRadius: 5, fontSize: 13, resize: 'vertical' as const, boxSizing: 'border-box' as const }} />
          </div>
          <button onClick={send} disabled={sending} style={{ padding: '9px 20px', background: sending ? '#93c5fd' : '#2563eb', color: '#fff', border: 'none', borderRadius: 6, cursor: sending ? 'not-allowed' : 'pointer', fontSize: 13, fontWeight: 600 }}>
            {sending ? 'Sending…' : 'Send Test Message'}
          </button>
          {testResult && <div style={{ marginTop: 12, padding: '10px 14px', background: testResult.startsWith('✓') ? '#f0fdf4' : '#fef2f2', borderRadius: 6, fontSize: 13, color: testResult.startsWith('✓') ? '#166534' : '#991b1b' }}>{testResult}</div>}
        </div>
      </section>

      <section style={{ marginTop: 32 }}>
        <h2 style={{ fontSize: 15, fontWeight: 600, color: '#374151', marginBottom: 12 }}>Quick Links</h2>
        <div style={{ display: 'flex', gap: 10 }}>
          {[{ label: 'Temporal UI', href: 'http://localhost:8080' }, { label: 'Prometheus', href: 'http://localhost:9090' }, { label: 'Brain Health', href: 'http://localhost:18081/health/deep' }].map(l => (
            <a key={l.href} href={l.href} target="_blank" rel="noopener noreferrer" style={{ padding: '7px 14px', background: '#fff', border: '1px solid #e5e7eb', borderRadius: 6, fontSize: 13, color: '#2563eb', textDecoration: 'none' }}>{l.label} ↗</a>
          ))}
        </div>
      </section>
    </main>
  );
}
