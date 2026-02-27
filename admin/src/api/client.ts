export async function apiGet(path: string): Promise<unknown> {
  const response = await fetch(path, { method: 'GET' });
  if (!response.ok) {
    throw new Error(`api error: ${response.status}`);
  }
  return response.json();
}
