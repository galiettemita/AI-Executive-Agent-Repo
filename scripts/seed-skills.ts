/* eslint-disable no-console */
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const disambiguationPath = path.resolve(root, 'config', 'skill-disambiguation.yaml');
const seedMigrationPath = path.resolve(root, 'migrations', '006_seed_skills.up.sql');

function requireFile(filePath: string): void {
  if (!fs.existsSync(filePath)) {
    console.error(`missing required file: ${filePath}`);
    process.exit(1);
  }
}

function parseSeedSkillIDs(sql: string): string[] {
  const sectionSplit = sql.split('), normalized AS (');
  if (sectionSplit.length < 2) {
    throw new Error('unable to locate seed CTE section in 006_seed_skills.up.sql');
  }

  const seedSection = sectionSplit[0];
  const unnestPattern = /unnest\(ARRAY\[(.*?)\]\)\s+AS\s+id/gs;
  const idPattern = /'([a-z0-9-]+)'/g;
  const ids = new Set<string>();
  let blockMatch: RegExpExecArray | null;
  while ((blockMatch = unnestPattern.exec(seedSection)) !== null) {
    const block = blockMatch[1];
    let idMatch: RegExpExecArray | null;
    while ((idMatch = idPattern.exec(block)) !== null) {
      ids.add(idMatch[1]);
    }
  }
  return Array.from(ids).sort();
}

function parseDisambiguationGroups(yaml: string): string[] {
  const groups = new Set<string>();
  for (const line of yaml.split('\n')) {
    const trimmed = line.trim();
    if (trimmed.startsWith('group:')) {
      groups.add(trimmed.replace('group:', '').trim());
    }
  }
  return Array.from(groups).sort();
}

function run(): void {
  requireFile(disambiguationPath);
  requireFile(seedMigrationPath);

  const disambiguationRaw = fs.readFileSync(disambiguationPath, 'utf8');
  const migrationRaw = fs.readFileSync(seedMigrationPath, 'utf8');

  const skillIDs = parseSeedSkillIDs(migrationRaw);
  const disambiguationGroups = parseDisambiguationGroups(disambiguationRaw);

  const payload = {
    checked_at: new Date().toISOString(),
    migration_file: seedMigrationPath,
    disambiguation_file: disambiguationPath,
    skill_count: skillIDs.length,
    disambiguation_group_count: disambiguationGroups.length,
    skills: skillIDs,
    disambiguation_groups: disambiguationGroups
  };

  if (skillIDs.length !== 153) {
    console.error(`skill seed count mismatch: expected 153, got ${skillIDs.length}`);
    process.exit(1);
  }
  if (disambiguationGroups.length !== 11) {
    console.error(`disambiguation group count mismatch: expected 11, got ${disambiguationGroups.length}`);
    process.exit(1);
  }

  const jsonOutputPathArg = process.argv.find((arg) => arg.startsWith('--json-output='));
  if (jsonOutputPathArg) {
    const outputPath = path.resolve(root, jsonOutputPathArg.replace('--json-output=', ''));
    fs.mkdirSync(path.dirname(outputPath), { recursive: true });
    fs.writeFileSync(outputPath, JSON.stringify(payload, null, 2));
    console.log(`wrote seed summary: ${outputPath}`);
  }

  if (process.argv.includes('--print-ids')) {
    for (const id of skillIDs) {
      console.log(id);
    }
    return;
  }

  console.log(JSON.stringify(payload, null, 2));
}

run();
