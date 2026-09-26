<template>
  <HelpSection :title="$t('help.aboutCapacitarr')" :delay="620" content-class="space-y-6">
    <div class="space-y-3">
      <p class="font-medium text-foreground">{{ $t('help.about.projectInfo') }}</p>
      <div class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2">
        <span class="text-muted-foreground">{{ $t('help.about.app') }}</span>
        <span class="text-foreground font-medium">{{ $t('help.about.appName') }}</span>

        <span class="text-muted-foreground">{{ $t('help.about.description') }}</span>
        <span class="text-foreground">{{ $t('help.about.appDescription') }}</span>

        <span class="text-muted-foreground">{{ $t('help.about.uiVersion') }}</span>
        <span class="text-foreground">
          v{{ uiVersion }}
          <template v-if="uiBuildDate">
            <span class="text-muted-foreground text-xs ml-1">({{ uiBuildDate }})</span>
          </template>
        </span>

        <span class="text-muted-foreground">{{ $t('help.about.apiVersion') }}</span>
        <span class="text-foreground">
          <template v-if="apiVersion">
            {{ apiVersion }}
            <template v-if="apiBuildDate">
              <span class="text-muted-foreground text-xs ml-1">({{ apiBuildDate }})</span>
            </template>
          </template>
          <template v-else> — </template>
        </span>

        <span class="text-muted-foreground">{{ $t('help.about.source') }}</span>
        <a
          href="https://github.com/Ghent/capacitarr"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1 text-primary hover:underline"
        >
          {{ $t('help.about.github') }}
          <ExternalLinkIcon class="w-3 h-3" />
        </a>

        <span class="text-muted-foreground">{{ $t('help.about.docs') }}</span>
        <a
          href="https://capacitarr.app"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1 text-primary hover:underline"
        >
          {{ $t('help.about.documentation') }}
          <ExternalLinkIcon class="w-3 h-3" />
        </a>

        <span class="text-muted-foreground">{{ $t('help.about.community') }}</span>
        <div class="flex items-center gap-3">
          <a
            href="https://discord.gg/fbFkND5qgt"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1.5 text-primary hover:underline"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="currentColor"
              class="w-3.5 h-3.5"
              aria-hidden="true"
            >
              <path
                d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028 14.09 14.09 0 0 0 1.226-1.994.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.095 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.095 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z"
              />
            </svg>
            {{ $t('help.about.discord') }}
            <ExternalLinkIcon class="w-3 h-3" />
          </a>
          <a
            href="https://www.reddit.com/r/capacitarr/"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1.5 text-primary hover:underline"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="currentColor"
              class="w-3.5 h-3.5"
              aria-hidden="true"
            >
              <path
                d="M12 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0zm5.01 4.744c.688 0 1.25.561 1.25 1.249a1.25 1.25 0 0 1-2.498.056l-2.597-.547-.8 3.747c1.824.07 3.48.632 4.674 1.488.308-.309.73-.491 1.207-.491.968 0 1.754.786 1.754 1.754 0 .716-.435 1.333-1.01 1.614a3.111 3.111 0 0 1 .042.52c0 2.694-3.13 4.87-7.004 4.87-3.874 0-7.004-2.176-7.004-4.87 0-.183.015-.366.043-.534A1.748 1.748 0 0 1 4.028 12c0-.968.786-1.754 1.754-1.754.463 0 .898.196 1.207.49 1.207-.883 2.878-1.43 4.744-1.487l.885-4.182a.342.342 0 0 1 .14-.197.35.35 0 0 1 .238-.042l2.906.617a1.214 1.214 0 0 1 1.108-.701zM9.25 12C8.561 12 8 12.562 8 13.25c0 .687.561 1.248 1.25 1.248.687 0 1.248-.561 1.248-1.249 0-.688-.561-1.249-1.249-1.249zm5.5 0c-.687 0-1.248.561-1.248 1.25 0 .687.561 1.248 1.249 1.248.688 0 1.249-.561 1.249-1.249 0-.687-.562-1.249-1.25-1.249zm-5.466 3.99a.327.327 0 0 0-.231.094.33.33 0 0 0 0 .463c.842.842 2.484.913 2.961.913.477 0 2.105-.056 2.961-.913a.361.361 0 0 0 .029-.463.33.33 0 0 0-.464 0c-.547.533-1.684.73-2.512.73-.828 0-1.979-.196-2.512-.73a.326.326 0 0 0-.232-.095z"
              />
            </svg>
            {{ $t('help.about.reddit') }}
            <ExternalLinkIcon class="w-3 h-3" />
          </a>
        </div>

        <span class="text-muted-foreground">{{ $t('help.about.donate') }}</span>
        <div class="space-y-1">
          <p class="text-xs text-foreground/70">
            {{ $t('help.about.donateHint') }}
          </p>
          <div class="flex items-center gap-3">
            <a
              href="https://uanimals.org/en/"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1.5 text-primary hover:underline"
            >
              <HeartIcon class="w-3.5 h-3.5" />
              {{ $t('help.about.uanimals') }}
              <ExternalLinkIcon class="w-3 h-3" />
            </a>
            <a
              href="https://www.aspca.org/ways-to-help"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1.5 text-primary hover:underline"
            >
              <PawPrintIcon class="w-3.5 h-3.5" />
              {{ $t('help.about.aspca') }}
              <ExternalLinkIcon class="w-3 h-3" />
            </a>
          </div>
        </div>

        <span class="text-muted-foreground">{{ $t('help.about.tipJar') }}</span>
        <div class="flex items-center gap-3">
          <a
            href="https://github.com/sponsors/ghent"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1.5 text-primary hover:underline"
          >
            {{ $t('help.about.githubSponsors') }}
            <ExternalLinkIcon class="w-3 h-3" />
          </a>
          <a
            href="https://ko-fi.com/ghent"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1.5 text-primary hover:underline"
          >
            {{ $t('help.about.kofi') }}
            <ExternalLinkIcon class="w-3 h-3" />
          </a>
          <a
            href="https://buymeacoffee.com/ghentgames"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1.5 text-primary hover:underline"
          >
            {{ $t('help.about.buyMeACoffee') }}
            <ExternalLinkIcon class="w-3 h-3" />
          </a>
        </div>

        <span class="text-muted-foreground">{{ $t('help.about.license') }}</span>
        <span class="text-foreground">{{ $t('help.about.licenseValue') }}</span>

        <span class="text-muted-foreground">{{ $t('help.about.author') }}</span>
        <span class="text-foreground font-medium">{{ $t('help.about.authorName') }}</span>

        <span class="text-muted-foreground">{{ $t('help.about.contributors') }}</span>
        <div class="flex flex-wrap items-center gap-1.5">
          <template v-if="contributors.length > 0">
            <span v-for="(name, idx) in contributors" :key="name" class="text-foreground"
              >{{ name
              }}<span v-if="idx < contributors.length - 1" class="text-muted-foreground/50"
                >,
              </span></span
            >
          </template>
          <span v-else class="text-muted-foreground italic">{{
            $t('help.about.noContributors')
          }}</span>
        </div>
      </div>
    </div>

    <div class="space-y-3">
      <p class="font-medium text-foreground">{{ $t('help.about.techStack') }}</p>
      <div class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2">
        <span class="text-muted-foreground">{{ $t('help.about.frontend') }}</span>
        <div class="flex flex-wrap gap-1.5">
          <UiBadge v-for="item in techStack.frontend" :key="item" variant="secondary">
            {{ item }}
          </UiBadge>
        </div>

        <span class="text-muted-foreground">{{ $t('help.about.backend') }}</span>
        <div class="flex flex-wrap gap-1.5">
          <UiBadge v-for="item in techStack.backend" :key="item" variant="secondary">
            {{ item }}
          </UiBadge>
        </div>

        <span class="text-muted-foreground">{{ $t('help.about.auth') }}</span>
        <div class="flex flex-wrap gap-1.5">
          <UiBadge v-for="item in techStack.auth" :key="item" variant="secondary">
            {{ item }}
          </UiBadge>
        </div>

        <span class="text-muted-foreground">{{ $t('help.about.infrastructure') }}</span>
        <div class="flex flex-wrap gap-1.5">
          <UiBadge v-for="item in techStack.infrastructure" :key="item" variant="secondary">
            {{ item }}
          </UiBadge>
        </div>
      </div>
    </div>

    <div class="space-y-3">
      <p class="font-medium text-foreground">{{ $t('help.about.creditsTitle') }}</p>
      <ul class="space-y-2 pl-1">
        <li v-for="credit in credits" :key="credit.nameKey" class="flex items-start gap-2">
          <span class="mt-1 w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
          <span
            ><strong class="text-foreground">{{ $t(credit.nameKey) }}</strong> —
            {{ $t(credit.descKey) }}</span
          >
        </li>
      </ul>
    </div>

    <div class="pt-4 border-t border-border/50">
      <p class="text-xs text-muted-foreground/70 leading-relaxed flex items-start gap-2">
        <img
          src="~/assets/images/flag-ua.svg"
          alt="Ukrainian flag"
          class="w-4 h-4 shrink-0 mt-0.5"
        />
        <span>{{ $t('help.about.ukraineNote') }}</span>
      </p>
    </div>
  </HelpSection>
</template>

<script setup lang="ts">
import { ExternalLinkIcon, HeartIcon, PawPrintIcon } from 'lucide-vue-next';

const { uiVersion, uiBuildDate, apiVersion, apiBuildDate } = useVersion();
const config = useRuntimeConfig();
const contributors = computed(() => (config.public.contributors as string[]) || []);

const techStack = {
  frontend: [
    'Vue 3',
    'Nuxt 4',
    'Tailwind CSS v4',
    'shadcn-vue',
    'ECharts',
    'Lucide Icons',
    'SSE (EventSource)',
  ],
  backend: [
    'Go 1.25',
    'Echo HTTP',
    'GORM + SQLite',
    'Service Layer',
    'Event Bus',
    'SSE Broadcaster',
  ],
  auth: ['JWT', 'bcrypt', 'API Key', 'Plex OAuth', 'Proxy Header'],
  infrastructure: ['Docker', 'Alpine Linux'],
};

const credits = [
  { nameKey: 'help.about.creditMaintainerr', descKey: 'help.about.creditMaintainerrDesc' },
  { nameKey: 'help.about.creditShadcn', descKey: 'help.about.creditShadcnDesc' },
  { nameKey: 'help.about.creditTailwind', descKey: 'help.about.creditTailwindDesc' },
  { nameKey: 'help.about.creditNuxt', descKey: 'help.about.creditNuxtDesc' },
  { nameKey: 'help.about.creditGeist', descKey: 'help.about.creditGeistDesc' },
  { nameKey: 'help.about.creditArr', descKey: 'help.about.creditArrDesc' },
];
</script>
