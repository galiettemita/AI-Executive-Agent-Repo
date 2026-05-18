import express, { Request, Response, NextFunction } from "express";
import { collectDefaultMetrics, register } from "prom-client";
import { loadSkillRegistry, SkillRegistry } from "./registry.js";

const app = express();
app.use(express.json({ limit: "1mb" }));

collectDefaultMetrics({ prefix: "hands_runtime_" });

let registry: SkillRegistry;

// ---------------------------------------------------------------------------
// Contract endpoints (D8 — strict versioned contract)
// ---------------------------------------------------------------------------

// 1. list_skills — returns all registered skill IDs and versions
app.get("/v1/skills", (_req: Request, res: Response) => {
  const skills = registry.listSkills();
  res.json({ skills, count: skills.length });
});

// 2. get_schema — returns the JSON schema for a specific skill
app.get("/v1/skills/:skillId/schema", (req: Request, res: Response) => {
  const { skillId } = req.params;
  const schema = registry.getSchema(skillId);
  if (!schema) {
    res.status(404).json({ error: "skill_not_found", skill_id: skillId });
    return;
  }
  res.json({ skill_id: skillId, schema });
});

// 3. execute_skill — execute a skill with validated input
app.post("/v1/skills/:skillId/execute", async (req: Request, res: Response) => {
  const { skillId } = req.params;
  const { input, receipt_id, workspace_id } = req.body;

  if (!receipt_id) {
    res.status(403).json({
      error: "AUTHORIZATION_REQUIRED",
      message: "receipt_id is required for skill execution",
    });
    return;
  }

  if (!workspace_id) {
    res.status(400).json({
      error: "WORKSPACE_REQUIRED",
      message: "workspace_id is required",
    });
    return;
  }

  const skill = registry.getSkill(skillId);
  if (!skill) {
    res.status(404).json({ error: "skill_not_found", skill_id: skillId });
    return;
  }

  try {
    const result = await skill.execute(input, { receipt_id, workspace_id });
    res.json({ skill_id: skillId, status: "success", result });
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err);
    res.status(500).json({
      skill_id: skillId,
      status: "error",
      error: message,
    });
  }
});

// 4. health — liveness/readiness probe
app.get("/healthz/live", (_req: Request, res: Response) => {
  res.json({ status: "alive" });
});

app.get("/healthz/ready", (_req: Request, res: Response) => {
  const ready = registry && registry.listSkills().length > 0;
  if (ready) {
    res.json({ status: "ready", skill_count: registry.listSkills().length });
  } else {
    res.status(503).json({ status: "not_ready" });
  }
});

app.get("/health", (_req: Request, res: Response) => {
  res.json({ status: "ok", skills_loaded: registry?.listSkills().length ?? 0 });
});

// 5. metrics — Prometheus metrics endpoint
app.get("/metrics", async (_req: Request, res: Response) => {
  res.set("Content-Type", register.contentType);
  res.end(await register.metrics());
});

// Error handler
app.use((err: Error, _req: Request, res: Response, _next: NextFunction) => {
  console.error("Unhandled error:", err);
  res.status(500).json({ error: "internal_error", message: err.message });
});

// ---------------------------------------------------------------------------
// Startup
// ---------------------------------------------------------------------------

const PORT = parseInt(process.env.HANDS_RUNTIME_PORT || "18089", 10);

async function main() {
  registry = await loadSkillRegistry();
  console.log(
    `[hands-runtime] Loaded ${registry.listSkills().length} skills`
  );

  app.listen(PORT, () => {
    console.log(`[hands-runtime] Listening on :${PORT}`);
    console.log(`[hands-runtime] Contract endpoints:`);
    console.log(`  GET  /v1/skills              — list_skills`);
    console.log(`  GET  /v1/skills/:id/schema   — get_schema`);
    console.log(`  POST /v1/skills/:id/execute  — execute_skill`);
    console.log(`  GET  /healthz/live           — health (liveness)`);
    console.log(`  GET  /healthz/ready          — health (readiness)`);
    console.log(`  GET  /metrics                — metrics`);
  });
}

main().catch((err) => {
  console.error("[hands-runtime] Fatal:", err);
  process.exit(1);
});
