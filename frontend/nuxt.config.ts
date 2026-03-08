import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';
import tailwindcss from '@tailwindcss/vite';
import pkg from './package.json';

// Parse CONTRIBUTORS.md at build time to embed contributor names
function parseContributors(): string[] {
  const contributorsPath = resolve(__dirname, '../CONTRIBUTORS.md');
  if (!existsSync(contributorsPath)) return [];
  const content = readFileSync(contributorsPath, 'utf-8');
  const names: string[] = [];
  // Match lines like "- **Name** ..." under any section
  for (const match of content.matchAll(/^- \*\*(.+?)\*\*/gm)) {
    if (match[1]) names.push(match[1].trim());
  }
  return names;
}

// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  modules: ['@nuxt/eslint', '@nuxtjs/i18n'],

  ssr: false,

  // Prevent Nuxt from scanning index.ts barrel files in ui/ directories
  // (shadcn-vue generates both .vue + index.ts which causes duplicate warnings)
  components: [
    {
      path: '~/components/ui',
      extensions: ['.vue'],
      prefix: 'Ui',
      pathPrefix: false,
    },
    {
      path: '~/components',
      extensions: ['.vue'],
      ignore: ['**/ui/**'],
    },
  ],

  devtools: {
    enabled: true,
  },

  app: {
    baseURL: process.env.NUXT_APP_BASE_URL || '/',
    buildAssetsDir: '/_assets/',
    pageTransition: { name: 'page', mode: 'out-in' },
    head: {
      script: [
        {
          innerHTML: `(function(){var t=localStorage.getItem('capacitarr-theme')||'violet';var m=localStorage.getItem('capacitarr-color-mode');document.documentElement.setAttribute('data-theme',t);if(m==='dark'||(!m&&matchMedia('(prefers-color-scheme:dark)').matches)){document.documentElement.classList.add('dark')}function showSplash(){var s=document.createElement('div');s.id='capacitarr-splash';s.innerHTML='<div class="icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5V19A9 3 0 0 0 21 19V5"/><path d="M3 12A9 3 0 0 0 21 12"/></svg></div><span class="label">Loading Capacitarr\\u2026</span>';document.body.prepend(s)}if(document.body){showSplash()}else{document.addEventListener('DOMContentLoaded',showSplash)}})();`,
          type: 'text/javascript',
        },
      ],
      style: [
        {
          innerHTML: `#capacitarr-splash{position:fixed;inset:0;z-index:9999;display:flex;align-items:center;justify-content:center;flex-direction:column;gap:1rem;background:var(--color-background,#0e0e14);transition:opacity .3s ease}.dark #capacitarr-splash{background:#0e0e14}#capacitarr-splash .icon{width:3rem;height:3rem;border-radius:.75rem;background:var(--color-primary,#7c3aed);display:flex;align-items:center;justify-content:center;animation:splash-pulse 1.5s ease-in-out infinite}#capacitarr-splash .icon svg{width:1.5rem;height:1.5rem;color:white}#capacitarr-splash .label{font-size:.875rem;color:var(--color-muted-foreground,#71717a);font-family:system-ui,sans-serif}@keyframes splash-pulse{0%,100%{opacity:.7;transform:scale(1)}50%{opacity:1;transform:scale(1.05)}}#capacitarr-splash.fade-out{opacity:0;pointer-events:none}`,
        },
      ],
      noscript: [{ innerHTML: '<style>#capacitarr-splash{display:none}</style>' }],
      link: [
        { rel: 'icon', type: 'image/svg+xml', href: 'favicon.svg' },
        { rel: 'manifest', href: 'site.webmanifest' },
      ],
    },
  },

  css: [
    '@fontsource/geist-sans/400.css',
    '@fontsource/geist-sans/500.css',
    '@fontsource/geist-sans/600.css',
    '@fontsource/geist-sans/700.css',
    '@fontsource/geist-mono/400.css',
    '@fontsource/geist-mono/500.css',
    '@fontsource/geist-mono/600.css',
    '~/assets/css/main.css',
  ],

  runtimeConfig: {
    public: {
      apiBaseUrl: process.env.NUXT_PUBLIC_API_BASE_URL ?? '',
      appVersion: pkg.version || '0.0.0',
      appBuildDate: new Date().toISOString(),
      contributors: parseContributors(),
    },
  },

  routeRules: {
    '/': { prerender: true },
  },

  compatibilityDate: '2025-01-15',

  vite: {
    // Spread the tailwindcss plugin array — @tailwindcss/vite returns Plugin[]
    // from its bundled vite types which differ from Nuxt's internal vite types
    // due to Rollup generic variance. The cast through unknown is the standard
    // TypeScript pattern for cross-package type alignment.
    plugins: tailwindcss() as unknown as NonNullable<
      Exclude<import('nuxt/schema').NuxtConfig['vite'], undefined>['plugins']
    >,
  },

  eslint: {
    config: {
      stylistic: {
        commaDangle: 'never',
        braceStyle: '1tbs',
      },
    },
  },

  i18n: {
    locales: [
      { code: 'en', name: 'English', file: 'en.json' },
      { code: 'es', name: 'Español', file: 'es.json' },
      { code: 'de', name: 'Deutsch', file: 'de.json' },
      { code: 'fr', name: 'Français', file: 'fr.json' },
      { code: 'pt-BR', name: 'Português (Brasil)', file: 'pt-BR.json' },
      { code: 'nl', name: 'Nederlands', file: 'nl.json' },
      { code: 'it', name: 'Italiano', file: 'it.json' },
      { code: 'pl', name: 'Polski', file: 'pl.json' },
      { code: 'sv', name: 'Svenska', file: 'sv.json' },
      { code: 'da', name: 'Dansk', file: 'da.json' },
      { code: 'nb', name: 'Norsk', file: 'nb.json' },
      { code: 'fi', name: 'Suomi', file: 'fi.json' },
      { code: 'ru', name: 'Русский', file: 'ru.json' },
      { code: 'uk', name: 'Українська', file: 'uk.json' },
      { code: 'cs', name: 'Čeština', file: 'cs.json' },
      { code: 'ro', name: 'Română', file: 'ro.json' },
      { code: 'hu', name: 'Magyar', file: 'hu.json' },
      { code: 'tr', name: 'Türkçe', file: 'tr.json' },
      { code: 'ja', name: '日本語', file: 'ja.json' },
      { code: 'ko', name: '한국어', file: 'ko.json' },
      { code: 'zh-CN', name: '简体中文', file: 'zh-CN.json' },
      { code: 'zh-TW', name: '繁體中文', file: 'zh-TW.json' },
    ],
    defaultLocale: 'en',
    lazy: true,
    langDir: '../app/locales',
    strategy: 'no_prefix',
    detectBrowserLanguage: {
      useCookie: true,
      cookieKey: 'capacitarr-locale',
      fallbackLocale: 'en',
    },
    bundle: {
      optimizeTranslationDirective: false,
    },
  },
});
