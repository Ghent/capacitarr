// @ts-check
import withNuxt from './.nuxt/eslint.config.mjs';
import prettierConfig from '@vue/eslint-config-prettier';

export default withNuxt(
  {
    ignores: ['app/types/generated/**'],
  },
  {
    rules: {
      'vue/multi-word-component-names': 'off',
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    },
  },
  // Prettier compat — must be last to disable all @stylistic formatting rules
  prettierConfig,
);
