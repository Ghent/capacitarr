<template>
  <header
    data-slot="navbar"
    class="sticky top-0 z-50 relative"
  >
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex items-center justify-between h-16">
        <!-- Brand -->
        <div class="flex items-center gap-6">
          <NuxtLink
            to="/"
            class="flex items-center gap-2.5 group"
          >
            <div
              data-slot="brand-icon"
              class="w-8 h-8 rounded-lg bg-primary flex items-center justify-center"
            >
              <component
                :is="DatabaseIcon"
                class="w-4.5 h-4.5 text-primary-foreground"
              />
            </div>
            <div class="flex flex-col">
              <span class="text-lg font-bold tracking-tight text-foreground leading-tight">
                Capacitarr
              </span>
              <span class="text-[10px] text-muted-foreground/50 leading-none font-mono">
                UI v{{ uiVersion }} · API {{ apiVersion || '···' }}
              </span>
              <span class="text-[10px] text-muted-foreground/40 leading-none italic mt-0.5">
                You paid for that disk, use it!
              </span>
            </div>
          </NuxtLink>

          <!-- Nav Links -->
          <nav class="hidden sm:flex items-center gap-1">
            <NuxtLink
              v-for="link in navLinks"
              :key="link.to"
              :to="link.to"
              class="px-3 py-1.5 rounded-lg text-sm font-medium transition-all duration-200"
              :class="[
                $route.path === link.to
                  ? 'text-primary bg-primary/10'
                  : 'text-muted-foreground hover:text-foreground hover:bg-accent'
              ]"
              :data-slot="$route.path === link.to ? 'nav-link-active' : undefined"
            >
              {{ link.label }}
            </NuxtLink>
          </nav>
        </div>

        <!-- Right side -->
        <div class="flex items-center gap-1">
          <!-- Engine Control -->
          <EngineControlPopover />

          <!-- Theme selector -->
          <UiDropdownMenu>
            <UiDropdownMenuTrigger as-child>
              <UiButton
                variant="ghost"
                size="icon"
                aria-label="Change theme"
              >
                <component
                  :is="PaletteIcon"
                  class="w-5 h-5"
                />
              </UiButton>
            </UiDropdownMenuTrigger>
            <UiDropdownMenuContent
              align="end"
              class="w-40"
            >
              <UiDropdownMenuLabel>Theme</UiDropdownMenuLabel>
              <UiDropdownMenuSeparator />
              <UiDropdownMenuItem
                v-for="t in themes"
                :key="t.id"
                class="flex items-center gap-2.5 cursor-pointer"
                @click="setTheme(t.id)"
              >
                <span
                  class="w-4 h-4 rounded-full border-2 shrink-0"
                  :class="theme === t.id ? 'border-primary' : 'border-transparent'"
                  :style="{ backgroundColor: themeSwatchColor(t) }"
                />
                <span>{{ t.label }}</span>
                <component
                  :is="CheckIcon"
                  v-if="theme === t.id"
                  class="w-4 h-4 ml-auto text-primary"
                />
              </UiDropdownMenuItem>
            </UiDropdownMenuContent>
          </UiDropdownMenu>

          <!-- Dark mode toggle -->
          <UiButton
            variant="ghost"
            size="icon"
            :aria-label="isDark ? 'Switch to light mode' : 'Switch to dark mode'"
            @click="toggle"
          >
            <component
              :is="isDark ? SunIcon : MoonIcon"
              class="w-5 h-5"
            />
          </UiButton>

          <!-- Help -->
          <UiButton
            variant="ghost"
            size="icon"
            as-child
          >
            <NuxtLink
              to="/help"
              aria-label="Help"
            >
              <component
                :is="CircleHelpIcon"
                class="w-5 h-5"
              />
            </NuxtLink>
          </UiButton>

          <!-- About -->
          <UiPopover>
            <UiPopoverTrigger as-child>
              <UiButton
                variant="ghost"
                size="icon"
                aria-label="About Capacitarr"
              >
                <component
                  :is="InfoIcon"
                  class="w-5 h-5"
                />
              </UiButton>
            </UiPopoverTrigger>
            <UiPopoverContent
              align="end"
              class="w-72"
            >
              <div class="space-y-3">
                <div>
                  <h4 class="font-semibold text-sm">
                    Capacitarr
                  </h4>
                  <span class="text-xs text-muted-foreground font-mono">v{{ uiVersion }}</span>
                </div>
                <p class="text-sm text-muted-foreground leading-snug">
                  Automated media library capacity management — score, rank, and clean up your *arr libraries.
                </p>
                <UiSeparator />
                <div class="space-y-1.5 text-xs text-muted-foreground">
                  <p>Built by the Capacitarr team</p>
                  <p>
                    Inspired by
                    <a
                      href="https://github.com/jorenn92/Maintainerr"
                      target="_blank"
                      rel="noopener"
                      class="text-primary hover:underline inline-flex items-center gap-0.5"
                    >
                      Maintainerr <component
                        :is="ExternalLinkIcon"
                        class="w-3 h-3"
                      />
                    </a>
                    and the
                    <a
                      href="https://wiki.servarr.com/"
                      target="_blank"
                      rel="noopener"
                      class="text-primary hover:underline inline-flex items-center gap-0.5"
                    >
                      *arr <component
                        :is="ExternalLinkIcon"
                        class="w-3 h-3"
                      />
                    </a>
                    ecosystem
                  </p>
                </div>
                <UiSeparator />
                <div class="flex items-center justify-between">
                  <span class="text-[10px] text-muted-foreground/60">Go · Nuxt · shadcn-vue · SQLite</span>
                  <a
                    href="https://github.com/capacitarr/capacitarr"
                    target="_blank"
                    rel="noopener"
                    class="text-xs text-primary hover:underline inline-flex items-center gap-1"
                  >
                    GitHub <component
                      :is="ExternalLinkIcon"
                      class="w-3 h-3"
                    />
                  </a>
                </div>
              </div>
            </UiPopoverContent>
          </UiPopover>

          <!-- Logout -->
          <UiButton
            variant="ghost"
            size="icon"
            aria-label="Logout"
            class="hover:text-destructive"
            @click="logout"
          >
            <component
              :is="LogOutIcon"
              class="w-5 h-5"
            />
          </UiButton>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { DatabaseIcon, MoonIcon, SunIcon, LogOutIcon, CircleHelpIcon, PaletteIcon, CheckIcon, InfoIcon, ExternalLinkIcon } from 'lucide-vue-next'
import type { ThemeMeta } from '~/composables/useTheme'

const { isDark, toggle } = useAppColorMode()
const { theme, setTheme, themes } = useTheme()
const { uiVersion, apiVersion } = useVersion()
const router = useRouter()
const authenticated = useCookie('authenticated')

const navLinks = [
  { to: '/', label: 'Dashboard' },
  { to: '/rules', label: 'Scoring Engine' },
  { to: '/audit', label: 'Audit Log' },
  { to: '/settings', label: 'Settings' }
]

function logout() {
  authenticated.value = null
  router.push('/login')
}

/** Map theme hue to a swatch color for the dropdown */
function themeSwatchColor(t: ThemeMeta): string {
  return `oklch(0.6 0.2 ${t.hue})`
}
</script>
