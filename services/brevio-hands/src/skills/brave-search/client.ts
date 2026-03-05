import type { BraveSearchInput, BraveSearchOutput, BraveSearchResult } from './types.js';

const RESULTS: BraveSearchResult[] = [
  {
    title: 'Executive assistant workflows',
    url: 'https://brave.example.com/executive-assistant-workflows',
    description: 'Practical workflows for managing executive tasks with AI.'
  },
  {
    title: 'Context window budgeting',
    url: 'https://brave.example.com/context-window-budgeting',
    description: 'How to allocate context budgets across tasks.'
  },
  {
    title: 'Operational playbook design',
    url: 'https://brave.example.com/operational-playbook-design',
    description: 'Design principles for repeatable execution playbooks.'
  }
];

export async function runClient(input: BraveSearchInput): Promise<BraveSearchOutput> {
  const terms = input.query.toLowerCase().split(/\s+/).filter((term) => term.length > 1);
  const results = RESULTS.filter((result) => {
    const haystack = `${result.title} ${result.description}`.toLowerCase();
    return terms.some((term) => haystack.includes(term));
  }).slice(0, input.max_results ?? 5);

  return {
    provider: 'brave-search',
    results
  };
}
