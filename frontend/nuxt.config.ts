import tailwindcss from '@tailwindcss/vite'

// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  modules: ['@nuxt/eslint'],

  ssr: false,

  devtools: {
    enabled: true
  },

  vite: {
    plugins: [tailwindcss()]
  },

  // Prevent Nuxt from scanning index.ts barrel files in ui/ directories
  // (shadcn-vue generates both .vue + index.ts which causes duplicate warnings)
  components: [
    {
      path: '~/components/ui',
      extensions: ['.vue'],
      prefix: 'Ui',
      pathPrefix: false
    },
    {
      path: '~/components',
      extensions: ['.vue'],
      ignore: ['**/ui/**']
    }
  ],

  app: {
    baseURL: process.env.NUXT_APP_BASE_URL || '/',
    buildAssetsDir: '/_assets/',
    pageTransition: { name: 'page', mode: 'out-in' },
    head: {
      script: [
        {
          innerHTML: `(function(){var t=localStorage.getItem('capacitarr-theme')||'violet';var m=localStorage.getItem('capacitarr-color-mode');document.documentElement.setAttribute('data-theme',t);if(m==='dark'||(!m&&matchMedia('(prefers-color-scheme:dark)').matches)){document.documentElement.classList.add('dark')}var s=document.createElement('div');s.id='capacitarr-splash';s.innerHTML='<div class="icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5V19A9 3 0 0 0 21 19V5"/><path d="M3 12A9 3 0 0 0 21 12"/></svg></div><span class="label">Loading Capacitarr\\u2026</span>';document.body.prepend(s)})();`,
          type: 'text/javascript'
        }
      ],
      style: [
        {
          innerHTML: `#capacitarr-splash{position:fixed;inset:0;z-index:9999;display:flex;align-items:center;justify-content:center;flex-direction:column;gap:1rem;background:var(--color-background,#0e0e14);transition:opacity .3s ease}.dark #capacitarr-splash{background:#0e0e14}#capacitarr-splash .icon{width:3rem;height:3rem;border-radius:.75rem;background:var(--color-primary,#7c3aed);display:flex;align-items:center;justify-content:center;animation:splash-pulse 1.5s ease-in-out infinite}#capacitarr-splash .icon svg{width:1.5rem;height:1.5rem;color:white}#capacitarr-splash .label{font-size:.875rem;color:var(--color-muted-foreground,#71717a);font-family:system-ui,sans-serif}@keyframes splash-pulse{0%,100%{opacity:.7;transform:scale(1)}50%{opacity:1;transform:scale(1.05)}}#capacitarr-splash.fade-out{opacity:0;pointer-events:none}`
        }
      ],
      noscript: [
        { innerHTML: '<style>#capacitarr-splash{display:none}</style>' }
      ]
    }
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
      apiBaseUrl: process.env.NUXT_PUBLIC_API_BASE_URL ?? ''
    }
  },

  routeRules: {
    '/': { prerender: true }
  },

  compatibilityDate: '2025-01-15',

  eslint: {
    config: {
      stylistic: {
        commaDangle: 'never',
        braceStyle: '1tbs'
      }
    }
  }
})
