// https://nuxt.com/docs/api/configuration/nuxt-config
import { env } from 'node:process'

// Base URL is configurable via SITE_BASE_URL environment variable.
// Defaults to '/' for the custom domain (capacitarr.app).
// Set SITE_BASE_URL=/software/capacitarr/ for a subdirectory deployment.
const baseURL = env.SITE_BASE_URL || '/'

export default defineNuxtConfig({
  modules: ['@nuxt/ui', '@nuxt/content'],

  css: ['~/assets/css/main.css'],

  app: {
    baseURL,
    pageTransition: { name: 'page', mode: 'out-in' },
    head: {
      htmlAttrs: { lang: 'en' },
    },
  },

  content: {
    build: {
      markdown: {
        toc: {
          searchDepth: 1,
        },
      },
    },
  },

  nitro: {
    prerender: {
      routes: ['/'],
      crawlLinks: true,
      autoSubfolderIndex: false,
      failOnError: false,
    },
  },

  icon: {
    provider: 'iconify',
  },

  compatibilityDate: '2024-07-11',
})
