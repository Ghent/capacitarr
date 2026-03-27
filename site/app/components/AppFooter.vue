<script setup lang="ts">
const { footer } = useAppConfig()
</script>

<template>
  <UFooter>
    <template #left>
      <div class="footer-brand">
        <span class="footer-logo-icon">
          <UIcon name="i-lucide-hard-drive" class="size-3.5" />
        </span>
        <span class="footer-credits">{{ footer.credits }}</span>
        <span class="footer-separator">·</span>
        <span class="footer-tagline">
          Built with
          <UIcon name="i-lucide-heart" class="size-3 text-rose-500 inline-block" />
          for the self-hosted community
        </span>
      </div>
    </template>

    <template #right>
      <div class="footer-links">
        <!-- Animal Rescue (primary) -->
        <div class="footer-link-group">
          <UIcon name="i-lucide-paw-print" class="size-3.5 text-amber-500" />
          <a
            href="https://uanimals.org/en/"
            target="_blank"
            rel="noopener noreferrer"
            class="footer-link"
          >
            UAnimals
          </a>
          <span class="footer-link-sep">·</span>
          <a
            href="https://www.aspca.org/ways-to-help"
            target="_blank"
            rel="noopener noreferrer"
            class="footer-link"
          >
            ASPCA
          </a>
        </div>

        <span class="footer-group-divider">|</span>

        <!-- Developer Support (secondary) -->
        <div class="footer-link-group">
          <UIcon name="i-lucide-heart-handshake" class="size-3.5 text-violet-500" />
          <a
            href="https://github.com/sponsors/ghent"
            target="_blank"
            rel="noopener noreferrer"
            class="footer-link"
          >
            Sponsor
          </a>
          <span class="footer-link-sep">·</span>
          <a
            href="https://ko-fi.com/ghent"
            target="_blank"
            rel="noopener noreferrer"
            class="footer-link"
          >
            Ko-fi
          </a>
          <span class="footer-link-sep">·</span>
          <a
            href="https://buymeacoffee.com/ghentgames"
            target="_blank"
            rel="noopener noreferrer"
            class="footer-link"
          >
            BMC
          </a>
        </div>

        <span class="footer-group-divider">|</span>

        <!-- Ukraine -->
        <span class="footer-ukraine" title="I stand with Ukraine">🇺🇦</span>

        <span class="footer-group-divider">|</span>

        <!-- GitHub -->
        <UButton
          icon="i-simple-icons-github"
          color="neutral"
          variant="ghost"
          to="https://github.com/Ghent/capacitarr"
          target="_blank"
          aria-label="Capacitarr on GitHub"
        />
      </div>
    </template>
  </UFooter>
</template>

<style scoped>
.footer-brand {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.footer-logo-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.375rem;
  height: 1.375rem;
  border-radius: 0.25rem;
  background: linear-gradient(135deg, var(--color-violet-500), var(--color-violet-600));
  color: white;
  flex-shrink: 0;
}

.footer-credits {
  font-size: 0.8125rem;
  color: var(--color-neutral-500);
}

.footer-separator {
  color: var(--color-neutral-300);
}

:root.dark .footer-separator {
  color: var(--color-neutral-700);
}

.footer-tagline {
  font-size: 0.75rem;
  color: var(--color-neutral-400);
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
}

.footer-links {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.footer-link-group {
  display: flex;
  align-items: center;
  gap: 0.375rem;
}

.footer-link {
  font-size: 0.75rem;
  color: var(--color-neutral-500);
  text-decoration: none;
  transition: color 0.2s;
}

.footer-link:hover {
  color: var(--color-primary-500);
}

.footer-link-sep {
  font-size: 0.625rem;
  color: var(--color-neutral-300);
}

:root.dark .footer-link-sep {
  color: var(--color-neutral-700);
}

.footer-group-divider {
  font-size: 0.75rem;
  color: var(--color-neutral-300);
  margin: 0 0.125rem;
}

:root.dark .footer-group-divider {
  color: var(--color-neutral-700);
}

.footer-ukraine {
  font-size: 1rem;
  line-height: 1;
  cursor: default;
}
</style>
