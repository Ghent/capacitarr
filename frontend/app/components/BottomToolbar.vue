<template>
  <footer
    v-motion
    v-bind="slideUpFromBottom"
    data-slot="bottom-toolbar"
    class="fixed bottom-0 inset-x-0 z-50 border-t border-border/50 backdrop-blur-xl bg-background/70 supports-[backdrop-filter]:bg-background/50"
    :style="{ paddingBottom: 'env(safe-area-inset-bottom, 0px)' }"
  >
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex items-center justify-center gap-1 h-12">
        <!-- Engine Control -->
        <EngineControlPopover side="top" />

        <!-- Version / Update Indicator -->
        <UiPopover>
          <UiPopoverTrigger as-child>
            <UiButton
              variant="ghost"
              size="icon"
              :aria-label="updateAvailable ? $t('update.title') : $t('update.upToDate')"
            >
              <component
                :is="ArrowUpCircleIcon"
                class="w-5 h-5"
                :class="updateAvailable ? 'update-breathe' : ''"
              />
            </UiButton>
          </UiPopoverTrigger>
          <UiPopoverContent align="center" side="top" class="w-72 p-4">
            <!-- Update available -->
            <div v-if="updateAvailable" class="space-y-3">
              <h4 class="text-sm font-semibold">{{ $t('update.title') }}</h4>
              <p class="text-xs text-muted-foreground">{{ $t('update.available') }}</p>
              <div class="space-y-1 text-sm">
                <div class="flex justify-between">
                  <span class="text-muted-foreground">{{ $t('update.currentVersion') }}</span>
                  <span class="font-mono">{{ apiVersion || uiVersion }}</span>
                </div>
                <div class="flex justify-between">
                  <span class="text-muted-foreground">{{ $t('update.latestVersion') }}</span>
                  <span class="font-mono text-primary">{{ latestVersion }}</span>
                </div>
              </div>
              <div class="flex items-center justify-between pt-1">
                <a
                  :href="releaseUrl"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="text-xs font-medium text-primary hover:underline"
                >
                  {{ $t('update.viewRelease') }}
                </a>
                <UiButton
                  variant="outline"
                  size="sm"
                  class="h-7 text-xs"
                  :disabled="checking"
                  @click="checkNow"
                >
                  <component
                    :is="checking ? LoaderCircleIcon : RefreshCwIcon"
                    class="w-3.5 h-3.5 mr-1"
                    :class="checking ? 'animate-spin' : ''"
                  />
                  {{ checking ? $t('update.checking') : $t('update.checkNow') }}
                </UiButton>
              </div>
            </div>
            <!-- Up to date -->
            <div v-else class="space-y-2">
              <div class="flex items-center gap-2">
                <component :is="CheckCircleIcon" class="w-4 h-4 text-primary" />
                <h4 class="text-sm font-semibold">{{ $t('update.upToDate') }}</h4>
              </div>
              <p class="text-xs text-muted-foreground">{{ $t('update.upToDateDesc') }}</p>
              <div class="text-sm">
                <div class="flex justify-between">
                  <span class="text-muted-foreground">{{ $t('update.currentVersion') }}</span>
                  <span class="font-mono">{{ apiVersion || uiVersion }}</span>
                </div>
              </div>
              <div class="flex justify-end pt-1">
                <UiButton
                  variant="outline"
                  size="sm"
                  class="h-7 text-xs"
                  :disabled="checking"
                  @click="checkNow"
                >
                  <component
                    :is="checking ? LoaderCircleIcon : RefreshCwIcon"
                    class="w-3.5 h-3.5 mr-1"
                    :class="checking ? 'animate-spin' : ''"
                  />
                  {{ checking ? $t('update.checking') : $t('update.checkNow') }}
                </UiButton>
              </div>
            </div>
          </UiPopoverContent>
        </UiPopover>

        <!-- Language selector -->
        <UiDropdownMenu>
          <UiDropdownMenuTrigger as-child>
            <UiButton variant="ghost" size="icon" :aria-label="$t('nav.changeLanguage')">
              <component :is="GlobeIcon" class="w-5 h-5" />
            </UiButton>
          </UiDropdownMenuTrigger>
          <UiDropdownMenuContent align="center" side="top" class="w-40">
            <UiDropdownMenuLabel>{{ $t('settings.language') }}</UiDropdownMenuLabel>
            <UiDropdownMenuSeparator />
            <UiDropdownMenuItem
              v-for="loc in locales"
              :key="typeof loc === 'string' ? loc : loc.code"
              class="flex items-center justify-between cursor-pointer"
              @click="setLocale(typeof loc === 'string' ? loc : loc.code)"
            >
              <span>{{ typeof loc === 'string' ? loc : loc.name }}</span>
              <component
                :is="CheckIcon"
                v-if="locale === (typeof loc === 'string' ? loc : loc.code)"
                class="w-4 h-4 text-primary"
              />
            </UiDropdownMenuItem>
          </UiDropdownMenuContent>
        </UiDropdownMenu>

        <!-- Theme & Mode selector -->
        <UiDropdownMenu>
          <UiDropdownMenuTrigger as-child>
            <UiButton variant="ghost" size="icon" :aria-label="$t('nav.changeTheme')">
              <component :is="PaletteIcon" class="w-5 h-5" />
            </UiButton>
          </UiDropdownMenuTrigger>
          <UiDropdownMenuContent align="center" side="top" class="w-44">
            <!-- Color Mode Section -->
            <UiDropdownMenuLabel>{{ $t('nav.mode') }}</UiDropdownMenuLabel>
            <UiDropdownMenuSeparator />
            <UiDropdownMenuItem
              class="flex items-center gap-2.5 cursor-pointer"
              @click="setMode('light')"
            >
              <component :is="SunIcon" class="w-4 h-4" />
              <span>{{ $t('nav.modeLight') }}</span>
              <component
                :is="CheckIcon"
                v-if="colorMode === 'light'"
                class="w-4 h-4 ml-auto text-primary"
              />
            </UiDropdownMenuItem>
            <UiDropdownMenuItem
              class="flex items-center gap-2.5 cursor-pointer"
              @click="setMode('dark')"
            >
              <component :is="MoonIcon" class="w-4 h-4" />
              <span>{{ $t('nav.modeDark') }}</span>
              <component
                :is="CheckIcon"
                v-if="colorMode === 'dark'"
                class="w-4 h-4 ml-auto text-primary"
              />
            </UiDropdownMenuItem>
            <UiDropdownMenuItem
              class="flex items-center gap-2.5 cursor-pointer"
              @click="setMode('system')"
            >
              <component :is="MonitorIcon" class="w-4 h-4" />
              <span>{{ $t('nav.modeSystem') }}</span>
              <component
                :is="CheckIcon"
                v-if="colorMode === 'system'"
                class="w-4 h-4 ml-auto text-primary"
              />
            </UiDropdownMenuItem>

            <!-- Theme Color Section -->
            <UiDropdownMenuSeparator />
            <UiDropdownMenuLabel>{{ $t('nav.theme') }}</UiDropdownMenuLabel>
            <UiDropdownMenuSeparator />
            <UiDropdownMenuItem
              v-for="themeOption in themes"
              :key="themeOption.id"
              class="flex items-center gap-2.5 cursor-pointer"
              @click="setTheme(themeOption.id)"
            >
              <span
                class="w-4 h-4 rounded-full border-2 shrink-0"
                :class="theme === themeOption.id ? 'border-primary' : 'border-transparent'"
                :style="{ backgroundColor: themeSwatchColor(themeOption) }"
              />
              <span>{{ themeOption.label }}</span>
              <component
                :is="CheckIcon"
                v-if="theme === themeOption.id"
                class="w-4 h-4 ml-auto text-primary"
              />
            </UiDropdownMenuItem>
          </UiDropdownMenuContent>
        </UiDropdownMenu>

        <!-- Donation / Support popover -->
        <UiPopover>
          <UiPopoverTrigger as-child>
            <UiButton variant="ghost" size="icon" :aria-label="$t('donate.ariaLabel')">
              <component :is="donateIcon" class="w-5 h-5" />
            </UiButton>
          </UiPopoverTrigger>
          <UiPopoverContent align="center" side="top" class="w-72 p-4">
            <div class="space-y-3">
              <!-- Header -->
              <div class="flex items-center gap-1.5">
                <component :is="PawPrintIcon" class="w-4 h-4 text-amber-500" />
                <h4 class="text-sm font-semibold">{{ $t('donate.title') }}</h4>
              </div>

              <!-- Message -->
              <p class="text-xs leading-relaxed text-muted-foreground">
                {{ $t('donate.message') }}
              </p>

              <!-- Charity links -->
              <div class="flex flex-col gap-1.5">
                <a
                  href="https://uanimals.org/en/"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="flex items-center gap-2 p-2 rounded-md hover:bg-accent transition-colors"
                >
                  <component :is="HeartIcon" class="w-4 h-4 text-amber-500 shrink-0" />
                  <div class="min-w-0">
                    <span class="block text-[13px] font-medium">{{
                      $t('donate.uanimalsName')
                    }}</span>
                    <span class="block text-[11px] text-muted-foreground">{{
                      $t('donate.uanimalsDesc')
                    }}</span>
                  </div>
                  <component
                    :is="ExternalLinkIcon"
                    class="w-3 h-3 ml-auto text-muted-foreground/50 shrink-0"
                  />
                </a>

                <a
                  href="https://www.aspca.org/ways-to-help"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="flex items-center gap-2 p-2 rounded-md hover:bg-accent transition-colors"
                >
                  <component :is="PawPrintIcon" class="w-4 h-4 text-orange-500 shrink-0" />
                  <div class="min-w-0">
                    <span class="block text-[13px] font-medium">{{ $t('donate.aspcaName') }}</span>
                    <span class="block text-[11px] text-muted-foreground">{{
                      $t('donate.aspcaDesc')
                    }}</span>
                  </div>
                  <component
                    :is="ExternalLinkIcon"
                    class="w-3 h-3 ml-auto text-muted-foreground/50 shrink-0"
                  />
                </a>
              </div>

              <!-- Separator -->
              <UiSeparator />

              <!-- Developer support -->
              <p
                class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/60"
              >
                {{ $t('donate.devHeading') }}
              </p>

              <div class="flex flex-wrap gap-x-3 gap-y-1">
                <a
                  href="https://github.com/sponsors/ghent"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-primary transition-colors"
                >
                  {{ $t('donate.githubSponsors') }}
                </a>
                <a
                  href="https://ko-fi.com/ghent"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-primary transition-colors"
                >
                  {{ $t('donate.kofi') }}
                </a>
                <a
                  href="https://buymeacoffee.com/ghentgames"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-primary transition-colors"
                >
                  {{ $t('donate.buyMeACoffee') }}
                </a>
              </div>
            </div>
          </UiPopoverContent>
        </UiPopover>

        <!-- Logout -->
        <UiButton
          variant="ghost"
          size="icon"
          :aria-label="$t('nav.logout')"
          class="hover:text-destructive"
          @click="logout"
        >
          <component :is="LogOutIcon" class="w-5 h-5" />
        </UiButton>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
import {
  MoonIcon,
  SunIcon,
  MonitorIcon,
  LogOutIcon,
  PaletteIcon,
  CheckIcon,
  CheckCircleIcon,
  GlobeIcon,
  ArrowUpCircleIcon,
  RefreshCwIcon,
  LoaderCircleIcon,
  CatIcon,
  DogIcon,
  HeartIcon,
  PawPrintIcon,
  ExternalLinkIcon,
} from 'lucide-vue-next';
import type { ThemeMeta } from '~/composables/useTheme';

const { slideUpFromBottom } = useMotionPresets();

/** Randomly pick a Cat or Dog icon for the donation button (chosen once per mount) */
const donateIcons = [CatIcon, DogIcon] as const;
const donateIcon = donateIcons[Math.floor(Math.random() * donateIcons.length)];

const { mode: colorMode, setMode } = useAppColorMode();
const { theme, setTheme, themes } = useTheme();
const { uiVersion, apiVersion, updateAvailable, latestVersion, releaseUrl, checking, checkNow } =
  useVersion();

const router = useRouter();
const authenticated = useAuthCookie();

const { locale, locales, setLocale } = useI18n();

function logout() {
  authenticated.value = null;
  router.push('/login');
}

/** Return the actual primary color for the theme swatch */
function themeSwatchColor(t: ThemeMeta): string {
  return t.primaryColor;
}
</script>
