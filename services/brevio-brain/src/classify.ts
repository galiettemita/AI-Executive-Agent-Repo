import {
  filterEnabledSkills,
  resolveEmailSkill,
  resolveMusicSkill,
  resolveNotesSkill,
  resolveTaskSkill
} from './catalog.js';
import type { IntentClassificationInput, IntentClassificationOutput, IntentEvidence } from './types.js';

interface IntentPattern {
  intent: string;
  keywords: string[];
  operation: string;
  candidateSkills: (input: IntentClassificationInput) => Array<string | undefined>;
}

const ACTION_CUES = [
  'and then',
  ' then ',
  ' after ',
  ' also ',
  ' plus ',
  ' and '
];

const PATTERNS: IntentPattern[] = [
  {
    intent: 'transport.flight_tracking',
    keywords: ['track flight', 'flight status', 'flight tracker', 'when does flight land', 'arrival gate'],
    operation: 'track',
    candidateSkills: (input) => [input.user_tier === 'free' ? 'flight-tracker' : 'aviationstack-flight-tracker']
  },
  {
    intent: 'transport.flight_search',
    keywords: ['find flight', 'search flight', 'book flight', 'cheapest flight', 'flights to'],
    operation: 'find',
    candidateSkills: () => ['aerobase-skill']
  },
  {
    intent: 'music.playback',
    keywords: ['play music', 'spotify', 'apple music', 'start playlist', 'play song', 'listen to'],
    operation: 'play',
    candidateSkills: (input) => [resolveMusicSkill(input.user_profile?.preferences ?? input.user_preferences, input.deployment_mode)]
  },
  {
    intent: 'calendar.schedule',
    keywords: ['schedule', 'calendar', 'meeting', 'remind me', 'create event', 'book time'],
    operation: 'create',
    candidateSkills: () => ['google-calendar']
  },
  {
    intent: 'shopping.research',
    keywords: ['find me', 'buy', 'shopping', 'price', 'under $', 'compare products'],
    operation: 'research',
    candidateSkills: () => ['shopping-expert']
  },
  {
    intent: 'tasks.manage',
    keywords: ['todo', 'task', 'checklist', 'add task', 'follow up', 'put this on my list'],
    operation: 'create',
    candidateSkills: (input) => [resolveTaskSkill(input.user_profile?.preferences ?? input.user_preferences)]
  },
  {
    intent: 'research.search',
    keywords: ['research', 'look up', 'find sources', 'search for', 'what are the latest'],
    operation: 'search',
    candidateSkills: () => ['tavily', 'brave-search', 'firecrawl-search']
  },
  {
    intent: 'notes.capture',
    keywords: ['save this note', 'note this', 'add to notes', 'capture this', 'write this down'],
    operation: 'create_note',
    candidateSkills: (input) => [resolveNotesSkill(input.user_profile?.preferences ?? input.user_preferences)]
  },
  {
    intent: 'finance.expense',
    keywords: ['spent', 'expense', 'budget', 'transaction', 'what did i spend'],
    operation: 'analyze',
    candidateSkills: () => ['smart-expense-tracker']
  },
  {
    intent: 'video.youtube',
    keywords: ['youtube', 'video', 'summarize this video', 'transcript from this video', 'youtube video'],
    operation: 'search',
    candidateSkills: () => ['youtube-api']
  },
  {
    intent: 'speech.transcribe',
    keywords: ['transcribe', 'speech to text', 'audio note', 'voice memo', 'diarize', 'speaker labels'],
    operation: 'transcribe',
    candidateSkills: () => ['asr', 'gemini-stt']
  },
  {
    intent: 'speech.synthesize',
    keywords: ['read this aloud', 'say this', 'text to speech', 'tts', 'speak this'],
    operation: 'synthesize',
    candidateSkills: (input) => [input.deployment_mode === 'local_mac' ? 'voice-wake-say' : 'openai-tts']
  },
  {
    intent: 'speech.conversation',
    keywords: ['voice chat', 'talk with me', 'audio conversation', 'speak with assistant'],
    operation: 'conversation',
    candidateSkills: () => ['vocal-chat']
  },
  {
    intent: 'image.analyze',
    keywords: ['describe this image', 'what is in this image', 'analyze this photo', 'caption this image', 'read this screenshot'],
    operation: 'analyze',
    candidateSkills: () => ['thinking-partner']
  },
  {
    intent: 'image.ocr',
    keywords: ['ocr', 'extract text from image', 'read the screenshot', 'read text in this photo'],
    operation: 'ocr',
    candidateSkills: () => ['thinking-partner']
  },
  {
    intent: 'document.parse',
    keywords: ['parse this pdf', 'extract text', 'extract table', 'document ocr', 'summarize this pdf'],
    operation: 'extract_text',
    candidateSkills: () => ['pdf-tools']
  },
  {
    intent: 'video.analyze',
    keywords: ['analyze this video', 'extract frames', 'summarize this video', 'what happens in this video'],
    operation: 'extract_frames',
    candidateSkills: () => ['video-frames']
  },
  {
    intent: 'camera.capture',
    keywords: ['take a picture', 'capture camera', 'camera snapshot', 'what do you see'],
    operation: 'capture',
    candidateSkills: () => ['camsnap']
  },
  {
    intent: 'media.generate',
    keywords: ['generate image', 'create image', 'make a video', 'generate video', 'upscale image'],
    operation: 'generate',
    candidateSkills: () => ['fal-ai', 'pollinations', 'krea-api', 'veo']
  },
  {
    intent: 'email.send',
    keywords: ['send email', 'email this', 'draft an email', 'reply to email', 'compose email'],
    operation: 'send',
    candidateSkills: (input) => [resolveEmailSkill(input.user_profile?.preferences ?? input.user_preferences, 'send', input.deployment_mode)]
  },
  {
    intent: 'email.search',
    keywords: ['search inbox', 'find email', 'search email', 'look up email', 'emails from', 'latest email'],
    operation: 'search',
    candidateSkills: (input) => [resolveEmailSkill(input.user_profile?.preferences ?? input.user_preferences, 'search', input.deployment_mode)]
  },
  {
    intent: 'places.search',
    keywords: ['near me', 'directions', 'navigate to', 'closest', 'find all', 'restaurants nearby'],
    operation: 'search',
    candidateSkills: () => ['local-places']
  }
];

function hasMultiActionSignals(message: string): boolean {
  const normalized = message.toLowerCase();
  return ACTION_CUES.some((cue) => normalized.includes(cue));
}

function scorePattern(normalized: string, pattern: IntentPattern): IntentEvidence {
  const matched = pattern.keywords.filter((keyword) => normalized.includes(keyword));
  const exactScore = matched.reduce((sum, keyword) => sum + (keyword.includes(' ') ? 1.3 : 1), 0);
  return {
    intent: pattern.intent,
    matched_keywords: matched,
    score: Number(exactScore.toFixed(2))
  };
}

function inferIntentFromMedia(input: IntentClassificationInput): IntentEvidence | undefined {
  const parts = input.content_parts ?? [];
  const assets = input.media_assets ?? parts.map((part) => part.media).filter((asset): asset is NonNullable<typeof asset> => Boolean(asset));
  const modalities = new Set<string>(parts.map((part) => part.type));
  for (const asset of assets) {
    const mime = asset.mime_type.toLowerCase();
    if (mime.startsWith('audio/')) modalities.add('audio');
    else if (mime.startsWith('image/')) modalities.add('image');
    else if (mime.startsWith('video/')) modalities.add('video');
    else if (mime === 'application/pdf' || mime.startsWith('text/')) modalities.add('document');
    else modalities.add('file');
  }
  if (modalities.has('audio')) {
    return { intent: 'speech.transcribe', matched_keywords: ['audio'], score: 1.8 };
  }
  if (modalities.has('video')) {
    return { intent: 'video.analyze', matched_keywords: ['video'], score: 1.8 };
  }
  if (modalities.has('image')) {
    return { intent: 'image.analyze', matched_keywords: ['image'], score: 1.6 };
  }
  if (modalities.has('document') || modalities.has('file')) {
    return { intent: 'document.parse', matched_keywords: ['document'], score: 1.6 };
  }
  return undefined;
}

function confidenceFor(bestScore: number, secondScore: number, keywordCount: number, clarificationRequired: boolean): number {
  if (bestScore <= 0 || keywordCount === 0) {
    return clarificationRequired ? 0.2 : 0.3;
  }

  const normalizedMatch = bestScore / Math.max(keywordCount, 1);
  const margin = Math.max(0, bestScore - secondScore);
  let confidence = 0.25 + normalizedMatch * 0.45 + Math.min(margin, 2) * 0.12;
  if (clarificationRequired) {
    confidence = Math.min(confidence, 0.6);
  }
  return Math.max(0.15, Math.min(0.98, Number(confidence.toFixed(2))));
}

function defaultClarification(text: string): string {
  if (text.trim() === '') {
    return 'What would you like me to help you do?';
  }
  return 'I can help, but I need a little more specificity about the action you want.';
}

function clarifyForMissingCapability(intent: string): string {
  if (intent.startsWith('email.')) {
    return 'I need an approved email connector and enabled skill before I can handle email.';
  }
  if (intent === 'music.playback') {
    return 'I need an approved music provider and enabled skill before I can start playback.';
  }
  return 'I need an explicit enabled skill before I can execute this request.';
}

export function classifyIntent(input: IntentClassificationInput): IntentClassificationOutput {
  const text = input.message_text.trim();
  const normalized = text.toLowerCase();
  const evidence = PATTERNS.map((pattern) => scorePattern(normalized, pattern));
  const mediaEvidence = inferIntentFromMedia(input);
  if (mediaEvidence) {
    evidence.push(mediaEvidence);
  }
  evidence.sort((left, right) => right.score - left.score);
  const bestEvidence = evidence[0];
  const secondEvidence = evidence[1];

  if (!bestEvidence || bestEvidence.score === 0) {
    return {
      intent: 'general.assistance',
      confidence: 0.2,
      skills: [],
      requires_decomposition: hasMultiActionSignals(text),
      reasoning: 'No reliable intent evidence matched; routed to clarification-first assistance.',
      clarification_required: true,
      blocked_skills: [],
      evidence: evidence.slice(0, 3),
      suggested_clarification: defaultClarification(text)
    };
  }

  const selectedPattern = PATTERNS.find((pattern) => pattern.intent === bestEvidence.intent);
  if (!selectedPattern) {
    throw new Error(`missing intent pattern for ${bestEvidence.intent}`);
  }

  const requestedSkills = selectedPattern.candidateSkills(input).filter((skill): skill is string => Boolean(skill));
  const { allowed, blocked } = filterEnabledSkills(requestedSkills, input.user_profile?.enabled_skills);
  const scoreMargin = bestEvidence.score - (secondEvidence?.score ?? 0);
  const missingCapability = requestedSkills.length === 0 || allowed.length === 0;
  const clarificationRequired = scoreMargin < 0.35 || missingCapability;
  const confidence = confidenceFor(bestEvidence.score, secondEvidence?.score ?? 0, selectedPattern.keywords.length, clarificationRequired);

  let reasoning = `Matched a supported ${bestEvidence.intent} request using deterministic policy-safe routing.`;
  if (missingCapability) {
    reasoning += ' Execution is paused until an approved skill is available for this intent.';
  } else if (clarificationRequired && scoreMargin < 0.35) {
    reasoning += ' Clarification is required before any external action is selected.';
  }

  let suggestedClarification: string | undefined;
  if (clarificationRequired) {
    if (missingCapability) {
      suggestedClarification = clarifyForMissingCapability(bestEvidence.intent);
    } else if (blocked.length > 0) {
      suggestedClarification = `Please enable one of these skills or choose another path: ${blocked.join(', ')}.`;
    } else {
      suggestedClarification = defaultClarification(text);
    }
  }

  return {
    intent: bestEvidence.intent,
    confidence,
    skills: allowed,
    requires_decomposition: hasMultiActionSignals(text),
    reasoning,
    clarification_required: clarificationRequired,
    blocked_skills: blocked,
    evidence: evidence.slice(0, 3),
    suggested_clarification: suggestedClarification,
    operation: selectedPattern.operation
  };
}
