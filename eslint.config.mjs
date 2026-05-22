// Flat-config ESLint setup for the monorepo.
//
// Replaces the legacy .eslintrc.cjs (incompatible with ESLint 9). The
// repository source is TypeScript, so typescript-eslint's recommended
// (syntactic-only) rules apply to **/*.ts. Pure JS files fall back to
// @eslint/js recommended.

import js from '@eslint/js';
import globals from 'globals';
import tseslint from 'typescript-eslint';

export default [
  {
    ignores: ['**/dist/**', '**/node_modules/**', '**/.turbo/**', '**/.pnpm-store/**']
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    languageOptions: {
      ecmaVersion: 2023,
      sourceType: 'module',
      globals: {
        ...globals.node
      }
    }
  }
];
