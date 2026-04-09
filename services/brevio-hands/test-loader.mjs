import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const loaderDir = path.dirname(fileURLToPath(import.meta.url));
const sharedRoot = path.resolve(loaderDir, '..', '..', 'packages', 'shared', 'src');

export async function resolve(specifier, context, defaultResolve) {
  if (specifier === '@brevio/shared') {
    return {
      shortCircuit: true,
      url: pathToFileURL(path.join(sharedRoot, 'index.ts')).href
    };
  }

  if ((specifier.startsWith('./') || specifier.startsWith('../')) && specifier.endsWith('.js')) {
    try {
      return await defaultResolve(specifier, context, defaultResolve);
    } catch {
      return defaultResolve(`${specifier.slice(0, -3)}.ts`, context, defaultResolve);
    }
  }

  return defaultResolve(specifier, context, defaultResolve);
}
