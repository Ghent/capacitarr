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
          innerHTML: `(function(){var t=localStorage.getItem('capacitarr-theme')||'violet';var m=localStorage.getItem('capacitarr-color-mode');document.documentElement.setAttribute('data-theme',t);if(m==='dark'||(!m&&matchMedia('(prefers-color-scheme:dark)').matches)){document.documentElement.classList.add('dark')}})();`,
          type: 'text/javascript'
        }
      ]
    }
  },

  css: [
    'geist/dist/geist-sans/style.css',
    'geist/dist/geist-mono/style.css',
    '~/assets/css/main.css',
  ],

  runtimeConfig: {
    public: {
      apiBaseUrl: process.env.NUXT_PUBLIC_API_BASE_URL || 'http://localhost:2187'
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
