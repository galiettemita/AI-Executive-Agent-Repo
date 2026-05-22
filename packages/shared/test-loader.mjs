export async function resolve(specifier, context, defaultResolve) {
  if ((specifier.startsWith('./') || specifier.startsWith('../')) && specifier.endsWith('.js')) {
    try {
      return await defaultResolve(specifier, context, defaultResolve);
    } catch (error) {
      return defaultResolve(`${specifier.slice(0, -3)}.ts`, context, defaultResolve);
    }
  }

  return defaultResolve(specifier, context, defaultResolve);
}
