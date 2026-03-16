import type { SkillCassette } from '../contract-validator.js';

/**
 * Cassette library for the 10 highest-priority skills.
 * Each cassette includes valid input/output matching the skill's declared schema,
 * plus HTTP mocks for the external API calls the skill makes.
 */
export const cassettes: SkillCassette[] = [
  // 1. Google Calendar — list events
  {
    skillId: 'google-calendar',
    description: 'List upcoming events returns valid events structure',
    input: {
      action: 'list',
    },
    expectedOutput: {
      action: 'list',
      calendar_id: 'primary',
      events: [],
      confirmation_required: false,
    },
    httpMocks: [{
      url: /googleapis\.com\/calendar\/v3/,
      method: 'GET',
      statusCode: 200,
      responseBody: { kind: 'calendar#events', items: [] },
    }],
  },

  // 2. Google Workspace — gmail list
  {
    skillId: 'google-workspace',
    description: 'Gmail list returns mail array',
    input: {
      action: 'gmail_list',
    },
    expectedOutput: {
      provider: 'google-workspace',
      action: 'gmail_list',
      mails: [],
    },
    httpMocks: [{
      url: /googleapis\.com\/gmail/,
      method: 'GET',
      statusCode: 200,
      responseBody: { messages: [] },
    }],
  },

  // 3. Slack — post message
  {
    skillId: 'slack',
    description: 'Post message returns channel and timestamp',
    input: {
      action: 'post_message',
      channel_id: 'C0123456',
      text: 'Hello from contract test',
    },
    expectedOutput: {
      provider: 'slack',
      action: 'post_message',
      post: { channel_id: 'C0123456', message_ts: '1234567890.123456', text: 'Hello from contract test' },
    },
    httpMocks: [{
      url: /slack\.com\/api\/chat\.postMessage/,
      method: 'POST',
      statusCode: 200,
      responseBody: { ok: true, channel: 'C0123456', ts: '1234567890.123456' },
    }],
  },

  // 4. Notion — search pages
  {
    skillId: 'notion',
    description: 'Search returns pages array',
    input: {
      action: 'search',
      query: 'meeting notes',
    },
    expectedOutput: {
      provider: 'notion',
      action: 'search',
      pages: [],
    },
    httpMocks: [{
      url: /api\.notion\.com\/v1\/search/,
      method: 'POST',
      statusCode: 200,
      responseBody: { results: [] },
    }],
  },

  // 5. Spotify — playback status
  {
    skillId: 'spotify',
    description: 'Status returns now_playing and summary',
    input: {
      action: 'status',
    },
    expectedOutput: {
      provider: 'spotify',
      action: 'status',
      now_playing: { track: 'Unknown', artist: 'Unknown', is_playing: false, volume_pct: 50, device: 'Unknown' },
      summary: 'No active playback session detected.',
    },
    httpMocks: [{
      url: /api\.spotify\.com\/v1\/me\/player/,
      method: 'GET',
      statusCode: 204,
      responseBody: {},
    }],
  },

  // 6. Todoist — list tasks
  {
    skillId: 'todoist',
    description: 'List tasks returns tasks array',
    input: {
      action: 'list',
    },
    expectedOutput: {
      provider: 'todoist_deterministic',
      action: 'list',
      tasks: [],
    },
    httpMocks: [{
      url: /api\.todoist\.com\/rest\/v2\/tasks/,
      method: 'GET',
      statusCode: 200,
      responseBody: [],
    }],
  },

  // 7. Jira — list issues
  {
    skillId: 'jira',
    description: 'Issue list returns issues array',
    input: {
      action: 'issue_list',
      project_key: 'BREV',
    },
    expectedOutput: {
      provider: 'jira',
      action: 'issue_list',
      issues: [],
    },
    httpMocks: [{
      url: /atlassian\.net\/rest\/api\/3\/search/,
      method: 'GET',
      statusCode: 200,
      responseBody: { issues: [] },
    }],
  },

  // 8. Brave Search — web search
  {
    skillId: 'brave-search',
    description: 'Search returns results array',
    input: {
      query: 'Brevio AI assistant',
    },
    expectedOutput: {
      provider: 'brave-search',
      results: [],
    },
    httpMocks: [{
      url: /api\.search\.brave\.com/,
      method: 'GET',
      statusCode: 200,
      responseBody: { web: { results: [] } },
    }],
  },

  // 9. HealthKit Sync — sync steps
  {
    skillId: 'healthkit-sync',
    description: 'Sync steps returns forwarded alias response',
    input: {
      action: 'sync_steps',
      days: 7,
    },
    expectedOutput: {
      provider: 'healthkit-sync',
      action: 'sync_steps',
      alias_target: 'healthkit-sync-apple',
      deprecated_alias: true,
      forwarded: true,
      summary: 'Forwarded to healthkit-sync-apple adapter.',
    },
  },

  // 10. Swiss Weather — forecast
  {
    skillId: 'swissweather',
    description: 'Forecast returns forecasts array and summary',
    input: {
      action: 'forecast',
      location: 'Zurich',
    },
    expectedOutput: {
      provider: 'swissweather',
      action: 'forecast',
      forecasts: [],
      summary: 'Weather forecast for Zurich.',
    },
    httpMocks: [{
      url: /swissweather|meteoswiss|openweathermap/,
      method: 'GET',
      statusCode: 200,
      responseBody: { forecasts: [] },
    }],
  },
];
