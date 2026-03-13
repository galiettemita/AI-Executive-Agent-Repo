import * as fs from "node:fs";
import * as path from "node:path";
import { logger } from "./logger.js";

export interface SkillContext {
  receipt_id: string;
  workspace_id: string;
}

export interface Skill {
  id: string;
  version: string;
  schema: Record<string, unknown>;
  execute: (
    input: Record<string, unknown>,
    ctx: SkillContext
  ) => Promise<unknown>;
}

export interface SkillSummary {
  id: string;
  version: string;
}

export interface SkillRegistry {
  listSkills: () => SkillSummary[];
  getSchema: (skillId: string) => Record<string, unknown> | null;
  getSkill: (skillId: string) => Skill | null;
}

/**
 * Loads skills from the src/skills/ directory.
 * Each skill directory must contain at minimum a schema.ts (or schema.json) file.
 */
export async function loadSkillRegistry(): Promise<SkillRegistry> {
  const skills = new Map<string, Skill>();

  const skillsDir = path.resolve(
    import.meta.dirname ?? path.dirname(new URL(import.meta.url).pathname),
    "skills"
  );

  if (!fs.existsSync(skillsDir)) {
    logger.warn("skills directory not found", { path: skillsDir });
    return createRegistry(skills);
  }

  const entries = fs.readdirSync(skillsDir, { withFileTypes: true });

  for (const entry of entries) {
    if (!entry.isDirectory()) continue;
    if (entry.name.startsWith("_")) continue; // skip templates

    const skillId = entry.name;
    const skillDir = path.join(skillsDir, skillId);

    // Try to load schema
    let schema: Record<string, unknown> = {};
    const schemaJsonPath = path.join(skillDir, "schema.json");
    if (fs.existsSync(schemaJsonPath)) {
      try {
        schema = JSON.parse(fs.readFileSync(schemaJsonPath, "utf-8"));
      } catch {
        logger.warn("failed to parse schema", { skill_id: skillId });
      }
    }

    skills.set(skillId, {
      id: skillId,
      version: "1.0.0",
      schema,
      execute: async (input, ctx) => {
        // Dynamic skill execution — loads the skill's index module
        try {
          const indexPath = path.join(skillDir, "index.js");
          if (fs.existsSync(indexPath)) {
            const mod = await import(indexPath);
            if (typeof mod.execute === "function") {
              return await mod.execute(input, ctx);
            }
          }
          return {
            status: "executed",
            skill_id: skillId,
            input_keys: Object.keys(input),
            workspace_id: ctx.workspace_id,
          };
        } catch (err) {
          throw new Error(
            `Skill ${skillId} execution failed: ${err instanceof Error ? err.message : String(err)}`
          );
        }
      },
    });
  }

  logger.info("skill registration complete", { count: skills.size });
  return createRegistry(skills);
}

function createRegistry(skills: Map<string, Skill>): SkillRegistry {
  return {
    listSkills: () =>
      Array.from(skills.values()).map((s) => ({
        id: s.id,
        version: s.version,
      })),
    getSchema: (skillId: string) => skills.get(skillId)?.schema ?? null,
    getSkill: (skillId: string) => skills.get(skillId) ?? null,
  };
}
