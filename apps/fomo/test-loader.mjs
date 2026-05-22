export async function resolve(specifier, context, defaultResolve) {
  if (specifier === '@brevio/shared') {
    return defaultResolve('../../../packages/shared/src/index.ts', context, defaultResolve);
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
