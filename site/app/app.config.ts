export default defineAppConfig({
  ui: {
    colors: {
      primary: 'violet',
      neutral: 'zinc',
    },
    footer: {
      slots: {
        root: 'border-t border-default',
        left: 'text-sm text-muted',
      },
    },
  },
  seo: {
    siteName: 'Capacitarr',
  },
  header: {
    title: 'Capacitarr',
    to: '/',
    search: true,
    colorMode: true,
    links: [{
      icon: 'i-simple-icons-github',
      to: 'https://github.com/Ghent/capacitarr',
      target: '_blank',
      'aria-label': 'GitHub',
    }],
  },
  footer: {
    credits: `© ${new Date().getFullYear()} Capacitarr`,
    colorMode: false,
  },
  toc: {
    title: 'On this page',
    bottom: {
      title: 'Resources',
      links: [{
        icon: 'i-simple-icons-github',
        label: 'View on GitHub',
        to: 'https://github.com/Ghent/capacitarr',
        target: '_blank',
      }],
    },
  },
})
