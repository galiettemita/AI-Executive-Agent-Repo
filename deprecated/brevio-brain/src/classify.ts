import type { IntentClassificationInput, IntentClassificationOutput } from './types.js';

interface IntentPattern {
  intent: string;
  skills: string[];
  keywords: string[];
  confidence: number;
}

const PATTERNS: IntentPattern[] = [
  {
    intent: 'transport.flight_tracking',
    skills: ['aviationstack-flight-tracker'],
    keywords: ['track flight', 'flight status', 'flight tracker', 'when does flight land'],
    confidence: 0.9
  },
  {
    intent: 'music.playback',
    skills: ['spotify-web-api'],
    keywords: ['play music', 'spotify', 'apple music', 'start playlist'],
    confidence: 0.84
  },
  {
    intent: 'calendar.schedule',
    skills: ['google-calendar'],
    keywords: ['schedule', 'calendar', 'meeting', 'remind me'],
    confidence: 0.82
  },
  {
    intent: 'shopping.research',
    skills: ['shopping-expert'],
    keywords: ['find me', 'buy', 'shopping', 'price', 'under $'],
    confidence: 0.8
  },
  {
    intent: 'tasks.manage',
    skills: ['todoist'],
    keywords: ['todo', 'task', 'checklist', 'add task'],
    confidence: 0.79
  },
  {
    intent: 'research.search',
    skills: ['tavily'],
    keywords: ['research', 'look up', 'find sources', 'search for'],
    confidence: 0.81
  },
  {
    intent: 'notes.capture',
    skills: ['apple-notes-skill'],
    keywords: ['save this note', 'note this', 'add to notes'],
    confidence: 0.82
  },
  {
    intent: 'finance.expense',
    skills: ['smart-expense-tracker'],
    keywords: ['spent', 'expense', 'budget', 'transaction'],
    confidence: 0.83
  },
  {
    intent: 'video.youtube',
    skills: ['youtube-api'],
    keywords: ['youtube', 'video', 'summarize this video'],
    confidence: 0.82
  }
];

function inferRequiresDecomposition(message: string): boolean {
  const normalized = message.toLowerCase();
  return normalized.includes(' and ') || normalized.includes(' then ') || normalized.includes('also ');
}

function intersectEnabled(skills: string[], enabled: string[] | undefined): string[] {
  if (!enabled || enabled.length === 0) {
    return skills;
  }
  const enabledSet = new Set(enabled);
  const filtered = skills.filter((skill) => enabledSet.has(skill));
  return filtered.length > 0 ? filtered : skills;
}

export function classifyIntent(input: IntentClassificationInput): IntentClassificationOutput {
  const text = input.message_text.trim();
  const normalized = text.toLowerCase();

  let bestPattern: IntentPattern | null = null;
  let bestScore = 0;

  for (const pattern of PATTERNS) {
    let score = 0;
    for (const keyword of pattern.keywords) {
      if (normalized.includes(keyword)) {
        score += 1;
      }
    }
    if (score > bestScore) {
      bestScore = score;
      bestPattern = pattern;
    }
  }

  if (!bestPattern || bestScore === 0) {
    return {
      intent: 'general.assistance',
      confidence: 0.55,
      skills: [],
      requires_decomposition: inferRequiresDecomposition(text),
      reasoning: 'No intent keywords matched; keyword fallback engaged.'
    };
  }

  const enabledSkills = input.user_profile?.enabled_skills;
  const resolvedSkills = intersectEnabled(bestPattern.skills, enabledSkills);

  return {
    intent: bestPattern.intent,
    confidence: bestPattern.confidence,
    skills: resolvedSkills.slice(0, 5),
    requires_decomposition: inferRequiresDecomposition(text),
    reasoning: `Matched keyword pattern for ${bestPattern.intent} with score ${bestScore}.`
  };
}
