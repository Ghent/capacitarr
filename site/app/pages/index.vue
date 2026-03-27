<script setup lang="ts">
useSeoMeta({
  titleTemplate: '',
  title: 'Capacitarr — Intelligent Media Library Management',
  ogTitle: 'Capacitarr — Intelligent Media Library Management',
  description: 'Automatically score, evaluate, and clean up your *arr stack. Connect Sonarr, Radarr, Lidarr, Readarr, Plex, Jellyfin, Emby, Tautulli, Jellystat, Tracearr, and Seerr.',
  ogDescription: 'Automatically score, evaluate, and clean up your *arr stack.',
})

const heroVisible = ref(false)
const featuresRef = ref<HTMLElement | null>(null)
const featuresVisible = ref(false)
const howItWorksRef = ref<HTMLElement | null>(null)
const howItWorksVisible = ref(false)
const ctaRef = ref<HTMLElement | null>(null)
const ctaVisible = ref(false)

const features = [
  {
    icon: 'i-lucide-bar-chart-3',
    title: 'Smart Scoring',
    description: 'Score media across multiple dimensions — age, file size, popularity, watch history, and more. Every score is transparent and fully explainable.',
    color: 'violet',
  },
  {
    icon: 'i-lucide-puzzle',
    title: 'Deep Integrations',
    description: 'Native support for 9+ services. Pull data from your *arr apps, media servers, and request managers into a single unified view.',
    color: 'blue',
  },
  {
    icon: 'i-lucide-layers',
    title: 'Cascading Rules',
    description: 'Build sophisticated rule chains with conditions, weights, and thresholds. Rules cascade and compose for complex cleanup policies.',
    color: 'amber',
  },
  {
    icon: 'i-lucide-shield-check',
    title: 'Safety First',
    description: 'Preview everything before it happens. Approval queue, safety guards, dry-run mode, and a complete audit trail ensure nothing is deleted by accident.',
    color: 'emerald',
  },
  {
    icon: 'i-lucide-radio',
    title: 'Real-Time Updates',
    description: 'Server-Sent Events push engine state, deletions, and activity to the browser instantly — no polling. 39 typed event types keep you informed.',
    color: 'rose',
  },
  {
    icon: 'i-lucide-container',
    title: 'Docker Native',
    description: 'Single container, zero external dependencies. Deploy in seconds with Docker Compose. Works on any platform, any architecture.',
    color: 'cyan',
  },
]

const steps = [
  {
    number: '01',
    title: 'Connect Your Stack',
    description: 'Point Capacitarr at your Sonarr, Radarr, Plex, and other services. API keys and URLs — that\'s all it needs.',
    icon: 'i-lucide-plug',
  },
  {
    number: '02',
    title: 'Configure Rules',
    description: 'Define what matters to you. Set scoring weights, thresholds, and conditions that match your library management philosophy.',
    icon: 'i-lucide-settings',
  },
  {
    number: '03',
    title: 'Review & Act',
    description: 'Capacitarr scores every item and shows you exactly what would be cleaned up — with full transparency on every decision.',
    icon: 'i-lucide-eye',
  },
]

function observe(el: Ref<HTMLElement | null>, flag: Ref<boolean>, threshold = 0.15) {
  onMounted(() => {
    if (!el.value) return
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          flag.value = true
          observer.disconnect()
        }
      },
      { threshold },
    )
    observer.observe(el.value)
  })
}

onMounted(() => {
  heroVisible.value = true
})

observe(featuresRef, featuresVisible)
observe(howItWorksRef, howItWorksVisible)
observe(ctaRef, ctaVisible)
</script>

<template>
  <div class="landing-page">
    <!-- ═══════════════════════════════════════════
         HERO SECTION
         ═══════════════════════════════════════════ -->
    <section class="hero-section">
      <!-- Animated background -->
      <div class="hero-bg">
        <div class="hero-gradient" />
        <div class="hero-grid" />
        <div class="hero-glow" />
      </div>

      <UContainer class="hero-content" :class="{ visible: heroVisible }">
        <div class="hero-badge">
          <UBadge color="violet" variant="subtle" size="lg">
            <UIcon name="i-lucide-sparkles" class="size-3.5 mr-1" />
            Open Source &amp; Self-Hosted
          </UBadge>
        </div>

        <h1 class="hero-title">
          Intelligent Media<br>
          Library <span class="hero-highlight">Management</span>
        </h1>

        <p class="hero-subtitle">
          Automatically score, evaluate, and clean up your *arr stack.
          <span class="hero-subtitle-muted">Connect everything. Control everything. Keep what matters.</span>
        </p>

        <div class="hero-actions">
          <UButton
            to="/docs/getting-started/quick-start"
            size="xl"
            trailing-icon="i-lucide-arrow-right"
            class="hero-cta-primary"
          >
            Quick Start
          </UButton>
          <UButton
            icon="i-simple-icons-github"
            color="neutral"
            variant="outline"
            size="xl"
            to="https://github.com/Ghent/capacitarr"
            target="_blank"
          >
            View on GitHub
          </UButton>
        </div>

        <!-- Hero screenshot gallery -->
        <div class="hero-gallery-wrapper">
          <div class="hero-screenshot-glow" />
          <ScreenshotGallery />
        </div>
      </UContainer>
    </section>

    <!-- ═══════════════════════════════════════════
         INTEGRATIONS STRIP
         ═══════════════════════════════════════════ -->
    <section class="integrations-section">
      <UContainer>
        <p class="section-eyebrow">Works With</p>
        <IntegrationStrip />
      </UContainer>
    </section>

    <!-- ═══════════════════════════════════════════
         FEATURES SECTION
         ═══════════════════════════════════════════ -->
    <section ref="featuresRef" class="features-section">
      <UContainer>
        <div class="section-header" :class="{ visible: featuresVisible }">
          <p class="section-eyebrow">Features</p>
          <h2 class="section-title">Everything you need to <span class="text-primary">manage capacity</span></h2>
          <p class="section-description">
            Capacitarr brings intelligence to media library management with powerful scoring, flexible rules, and deep integrations.
          </p>
        </div>

        <div class="features-grid">
          <div
            v-for="(feature, index) in features"
            :key="feature.title"
            class="feature-card"
            :class="{ visible: featuresVisible }"
            :style="{ '--delay': `${index * 100}ms` }"
          >
            <div class="feature-icon" :class="`feature-icon-${feature.color}`">
              <UIcon :name="feature.icon" class="size-5" />
            </div>
            <h3 class="feature-title">{{ feature.title }}</h3>
            <p class="feature-description">{{ feature.description }}</p>
          </div>
        </div>
      </UContainer>
    </section>

    <!-- ═══════════════════════════════════════════
         STATS SECTION
         ═══════════════════════════════════════════ -->
    <section class="stats-section">
      <UContainer>
        <AnimatedStats />
      </UContainer>
    </section>

    <!-- ═══════════════════════════════════════════
         COMPARISON SECTION
         ═══════════════════════════════════════════ -->
    <section class="comparison-section">
      <UContainer>
        <div class="section-header visible">
          <p class="section-eyebrow">Why Capacitarr?</p>
          <h2 class="section-title">From chaos to <span class="text-primary">clarity</span></h2>
        </div>
        <ComparisonSection />
      </UContainer>
    </section>

    <!-- ═══════════════════════════════════════════
         FAQ SECTION
         ═══════════════════════════════════════════ -->
    <section class="faq-section">
      <UContainer>
        <div class="section-header visible">
          <p class="section-eyebrow">FAQ</p>
          <h2 class="section-title">Frequently asked <span class="text-primary">questions</span></h2>
        </div>
        <FaqSection />
      </UContainer>
    </section>

    <!-- ═══════════════════════════════════════════
         ABOUT SECTION
         ═══════════════════════════════════════════ -->
    <section class="about-section">
      <UContainer>
        <div class="about-content">
          <div class="about-card">
            <div class="about-header">
              <UIcon name="i-lucide-info" class="size-5 text-primary" />
              <h3 class="about-title">About Capacitarr</h3>
            </div>
            <p class="about-text">
              Capacitarr is free, open-source software created by <strong>Ghent Starshadow</strong>.
              Licensed under PolyForm Noncommercial 1.0.0 + CLA. Built with Go, Nuxt 4, and SQLite.
            </p>
            <div class="about-ukraine">
              <img src="/flag-ua.svg" alt="Ukrainian flag" class="ukraine-flag">
              <p class="about-ukraine-text"><strong>I stand with Ukraine.</strong> This project is built with the belief that freedom, sovereignty, and self-determination matter — for people and for software.</p>
            </div>
            <div class="about-support">
              <p class="about-support-heading">
                <UIcon name="i-lucide-heart" class="size-4 text-rose-500" />
                Support Animal Rescue
              </p>
              <p class="about-support-text">
                Capacitarr is free software. If it saves you time, <strong>please consider donating to animal rescue instead of supporting the developer directly.</strong>
              </p>
              <div class="about-support-links">
                <UButton
                  to="https://uanimals.org/en/"
                  target="_blank"
                  icon="i-lucide-heart"
                  color="primary"
                  size="sm"
                >
                  UAnimals 🇺🇦
                </UButton>
                <UButton
                  to="https://www.aspca.org/ways-to-help"
                  target="_blank"
                  icon="i-lucide-paw-print"
                  color="primary"
                  size="sm"
                >
                  ASPCA
                </UButton>
              </div>
              <p class="about-support-dev">
                Or support the developer:
                <a href="https://github.com/sponsors/ghent" target="_blank" rel="noopener noreferrer">GitHub Sponsors</a> ·
                <a href="https://ko-fi.com/ghent" target="_blank" rel="noopener noreferrer">Ko-fi</a> ·
                <a href="https://buymeacoffee.com/ghentgames" target="_blank" rel="noopener noreferrer">Buy Me a Coffee</a>
              </p>
            </div>
          </div>
        </div>
      </UContainer>
    </section>

    <!-- ═══════════════════════════════════════════
         HOW IT WORKS
         ═══════════════════════════════════════════ -->
    <section ref="howItWorksRef" class="how-it-works-section">
      <UContainer>
        <div class="section-header" :class="{ visible: howItWorksVisible }">
          <p class="section-eyebrow">How It Works</p>
          <h2 class="section-title">Up and running in <span class="text-primary">minutes</span></h2>
        </div>

        <div class="steps-grid">
          <div
            v-for="(step, index) in steps"
            :key="step.number"
            class="step-card"
            :class="{ visible: howItWorksVisible }"
            :style="{ '--delay': `${index * 150}ms` }"
          >
            <div class="step-number">{{ step.number }}</div>
            <div class="step-icon">
              <UIcon :name="step.icon" class="size-6" />
            </div>
            <h3 class="step-title">{{ step.title }}</h3>
            <p class="step-description">{{ step.description }}</p>
          </div>
        </div>

        <!-- Animated terminal -->
        <AnimatedTerminal />
      </UContainer>
    </section>

    <!-- ═══════════════════════════════════════════
         FINAL CTA
         ═══════════════════════════════════════════ -->
    <section ref="ctaRef" class="cta-section">
      <div class="cta-bg">
        <div class="cta-gradient" />
      </div>
      <UContainer>
        <div class="cta-content" :class="{ visible: ctaVisible }">
          <h2 class="cta-title">Ready to take control of your library?</h2>
          <p class="cta-description">
            Capacitarr is free, open-source, and self-hosted. Get started in under a minute.
          </p>
          <div class="cta-actions">
            <UButton
              to="/docs/getting-started/quick-start"
              size="xl"
              trailing-icon="i-lucide-arrow-right"
            >
              Quick Start Guide
            </UButton>
            <UButton
              to="/docs"
              color="neutral"
              variant="outline"
              size="xl"
            >
              Browse Documentation
            </UButton>
          </div>
        </div>
      </UContainer>
    </section>
  </div>
</template>

<style scoped>
/* ═══════════════════════════════════════════
   HERO SECTION
   ═══════════════════════════════════════════ */
.hero-section {
  position: relative;
  overflow: hidden;
  padding-top: 4rem;
  padding-bottom: 2rem;
}

.hero-bg {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.hero-gradient {
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse 80% 60% at 50% -10%, rgba(139, 92, 246, 0.15), transparent 70%);
}

:root.dark .hero-gradient {
  background: radial-gradient(ellipse 80% 60% at 50% -10%, rgba(139, 92, 246, 0.2), transparent 70%);
}

.hero-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(to right, var(--color-neutral-200) 1px, transparent 1px),
    linear-gradient(to bottom, var(--color-neutral-200) 1px, transparent 1px);
  background-size: 4rem 4rem;
  opacity: 0.3;
  mask-image: radial-gradient(ellipse 60% 50% at 50% 0%, black, transparent 70%);
}

:root.dark .hero-grid {
  background-image:
    linear-gradient(to right, var(--color-neutral-800) 1px, transparent 1px),
    linear-gradient(to bottom, var(--color-neutral-800) 1px, transparent 1px);
  opacity: 0.4;
}

.hero-glow {
  position: absolute;
  top: -20%;
  left: 50%;
  transform: translateX(-50%);
  width: 40rem;
  height: 40rem;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(139, 92, 246, 0.08), transparent 60%);
  animation: pulse-glow 6s ease-in-out infinite;
}

:root.dark .hero-glow {
  background: radial-gradient(circle, rgba(139, 92, 246, 0.12), transparent 60%);
}

@keyframes pulse-glow {
  0%, 100% { opacity: 0.5; transform: translateX(-50%) scale(1); }
  50% { opacity: 1; transform: translateX(-50%) scale(1.1); }
}

.hero-content {
  position: relative;
  z-index: 1;
  text-align: center;
  opacity: 0;
  transform: translateY(2rem);
  transition: opacity 0.8s ease, transform 0.8s ease;
}

.hero-content.visible {
  opacity: 1;
  transform: translateY(0);
}

.hero-badge {
  margin-bottom: 1.5rem;
}

.hero-title {
  font-size: clamp(2.5rem, 6vw, 4rem);
  font-weight: 800;
  line-height: 1.1;
  letter-spacing: -0.03em;
  color: var(--color-neutral-900);
  margin-bottom: 1.25rem;
}

:root.dark .hero-title {
  color: var(--color-neutral-50);
}

.hero-highlight {
  background: linear-gradient(135deg, var(--color-violet-500), var(--color-violet-400), var(--color-purple-400));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.hero-subtitle {
  font-size: 1.125rem;
  line-height: 1.6;
  color: var(--color-neutral-600);
  max-width: 36rem;
  margin: 0 auto 2rem;
}

:root.dark .hero-subtitle {
  color: var(--color-neutral-400);
}

.hero-subtitle-muted {
  display: block;
  color: var(--color-neutral-400);
  margin-top: 0.25rem;
}

:root.dark .hero-subtitle-muted {
  color: var(--color-neutral-500);
}

.hero-actions {
  display: flex;
  gap: 0.75rem;
  justify-content: center;
  flex-wrap: wrap;
  margin-bottom: 3rem;
}

.hero-cta-primary {
  box-shadow: 0 0 20px -5px rgba(139, 92, 246, 0.4);
}

/* Hero gallery wrapper */
.hero-gallery-wrapper {
  position: relative;
  max-width: 56rem;
  margin: 0 auto;
}

.hero-screenshot-glow {
  position: absolute;
  inset: -2rem;
  border-radius: 1.5rem;
  background: radial-gradient(ellipse at center, rgba(139, 92, 246, 0.1), transparent 70%);
  filter: blur(20px);
}

/* ═══════════════════════════════════════════
   SHARED SECTION STYLES
   ═══════════════════════════════════════════ */
.section-eyebrow {
  text-align: center;
  font-size: 0.8125rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--color-violet-500);
  margin-bottom: 0.75rem;
}

.section-header {
  text-align: center;
  margin-bottom: 3rem;
  opacity: 0;
  transform: translateY(1.5rem);
  transition: opacity 0.6s ease, transform 0.6s ease;
}

.section-header.visible {
  opacity: 1;
  transform: translateY(0);
}

.section-title {
  font-size: clamp(1.75rem, 4vw, 2.5rem);
  font-weight: 700;
  letter-spacing: -0.025em;
  color: var(--color-neutral-900);
  margin-bottom: 0.75rem;
}

:root.dark .section-title {
  color: var(--color-neutral-50);
}

.section-description {
  font-size: 1.0625rem;
  color: var(--color-neutral-500);
  max-width: 32rem;
  margin: 0 auto;
  line-height: 1.6;
}

/* ═══════════════════════════════════════════
   INTEGRATIONS SECTION
   ═══════════════════════════════════════════ */
.integrations-section {
  padding: 3rem 0;
  border-top: 1px solid var(--color-neutral-200);
  border-bottom: 1px solid var(--color-neutral-200);
  background: var(--color-neutral-50);
}

:root.dark .integrations-section {
  border-color: var(--color-neutral-800);
  background: var(--color-neutral-950);
}

/* ═══════════════════════════════════════════
   FEATURES SECTION
   ═══════════════════════════════════════════ */
.features-section {
  padding: 5rem 0;
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1.5rem;
}

@media (max-width: 768px) {
  .features-grid {
    grid-template-columns: 1fr;
  }
}

@media (min-width: 769px) and (max-width: 1024px) {
  .features-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

.feature-card {
  padding: 1.5rem;
  border-radius: 0.75rem;
  border: 1px solid var(--color-neutral-200);
  background: var(--color-neutral-50);
  opacity: 0;
  transform: translateY(1.5rem);
  transition:
    opacity 0.5s ease var(--delay),
    transform 0.5s ease var(--delay),
    border-color 0.3s ease,
    box-shadow 0.3s ease,
    background 0.3s ease;
}

:root.dark .feature-card {
  border-color: var(--color-neutral-800);
  background: var(--color-neutral-900);
}

.feature-card.visible {
  opacity: 1;
  transform: translateY(0);
}

.feature-card:hover {
  border-color: transparent;
  box-shadow: 0 8px 30px -10px rgba(139, 92, 246, 0.15);
  transform: translateY(-2px);
  background:
    linear-gradient(var(--color-neutral-50), var(--color-neutral-50)) padding-box,
    linear-gradient(135deg, var(--color-violet-400), var(--color-purple-400), var(--color-blue-400)) border-box;
}

:root.dark .feature-card:hover {
  border-color: transparent;
  box-shadow: 0 8px 30px -10px rgba(139, 92, 246, 0.2);
  background:
    linear-gradient(var(--color-neutral-900), var(--color-neutral-900)) padding-box,
    linear-gradient(135deg, var(--color-violet-500), var(--color-purple-500), var(--color-blue-500)) border-box;
}

.feature-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.5rem;
  height: 2.5rem;
  border-radius: 0.5rem;
  margin-bottom: 0.75rem;
}

.feature-icon-violet {
  background: var(--color-violet-100);
  color: var(--color-violet-600);
}
:root.dark .feature-icon-violet {
  background: var(--color-violet-950);
  color: var(--color-violet-400);
}

.feature-icon-blue {
  background: var(--color-blue-100);
  color: var(--color-blue-600);
}
:root.dark .feature-icon-blue {
  background: var(--color-blue-950);
  color: var(--color-blue-400);
}

.feature-icon-amber {
  background: var(--color-amber-100);
  color: var(--color-amber-600);
}
:root.dark .feature-icon-amber {
  background: var(--color-amber-950);
  color: var(--color-amber-400);
}

.feature-icon-emerald {
  background: var(--color-emerald-100);
  color: var(--color-emerald-600);
}
:root.dark .feature-icon-emerald {
  background: var(--color-emerald-950);
  color: var(--color-emerald-400);
}

.feature-icon-rose {
  background: var(--color-rose-100);
  color: var(--color-rose-600);
}
:root.dark .feature-icon-rose {
  background: var(--color-rose-950);
  color: var(--color-rose-400);
}

.feature-icon-cyan {
  background: var(--color-cyan-100);
  color: var(--color-cyan-600);
}
:root.dark .feature-icon-cyan {
  background: var(--color-cyan-950);
  color: var(--color-cyan-400);
}

.feature-title {
  font-size: 1.0625rem;
  font-weight: 600;
  color: var(--color-neutral-900);
  margin-bottom: 0.375rem;
}

:root.dark .feature-title {
  color: var(--color-neutral-100);
}

.feature-description {
  font-size: 0.875rem;
  line-height: 1.6;
  color: var(--color-neutral-500);
}

/* ═══════════════════════════════════════════
   STATS SECTION
   ═══════════════════════════════════════════ */
.stats-section {
  padding: 3rem 0 5rem;
}

/* ═══════════════════════════════════════════
   ABOUT SECTION
   ═══════════════════════════════════════════ */
.about-section {
  padding: 3rem 0;
  background: var(--color-neutral-50);
  border-top: 1px solid var(--color-neutral-200);
  border-bottom: 1px solid var(--color-neutral-200);
}

:root.dark .about-section {
  background: var(--color-neutral-950);
  border-color: var(--color-neutral-800);
}

.about-content {
  max-width: 40rem;
  margin: 0 auto;
}

.about-card {
  padding: 1.5rem;
  border-radius: 0.75rem;
  border: 1px solid var(--color-neutral-200);
  background: white;
}

:root.dark .about-card {
  border-color: var(--color-neutral-800);
  background: var(--color-neutral-900);
}

.about-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}

.about-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--color-neutral-900);
}

:root.dark .about-title {
  color: var(--color-neutral-100);
}

.about-text {
  font-size: 0.875rem;
  line-height: 1.6;
  color: var(--color-neutral-500);
  margin-bottom: 1rem;
}

.about-support {
  padding: 1rem 0;
  border-top: 1px solid var(--color-neutral-200);
}

:root.dark .about-support {
  border-top-color: var(--color-neutral-800);
}

.about-support-heading {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--color-neutral-700);
  margin-bottom: 0.375rem;
}

:root.dark .about-support-heading {
  color: var(--color-neutral-300);
}

.about-support-text {
  font-size: 0.8125rem;
  line-height: 1.5;
  color: var(--color-neutral-500);
  margin-bottom: 0.75rem;
}

.about-support-links {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}

.about-support-dev {
  font-size: 0.75rem;
  color: var(--color-neutral-400);
}

.about-support-dev a {
  color: var(--color-primary-500);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.about-support-dev a:hover {
  color: var(--color-primary-400);
}

.about-ukraine {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 0.875rem;
  border-radius: 0.5rem;
  border-left: 3px solid #0057B7;
  background: linear-gradient(135deg, rgba(0, 87, 183, 0.05), rgba(255, 215, 0, 0.05));
  margin-bottom: 1rem;
}

:root.dark .about-ukraine {
  background: linear-gradient(135deg, rgba(0, 87, 183, 0.1), rgba(255, 215, 0, 0.08));
}

.ukraine-flag {
  width: 1.5rem;
  height: 1.5rem;
  margin-top: 0.125rem;
  flex-shrink: 0;
}

.about-ukraine-text {
  font-size: 0.875rem;
  line-height: 1.6;
  color: var(--color-neutral-600);
}

:root.dark .about-ukraine-text {
  color: var(--color-neutral-300);
}

/* ═══════════════════════════════════════════
   HOW IT WORKS
   ═══════════════════════════════════════════ */
.how-it-works-section {
  padding: 5rem 0;
}

.steps-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 2rem;
  margin-bottom: 3rem;
}

@media (max-width: 768px) {
  .steps-grid {
    grid-template-columns: 1fr;
  }
}

.step-card {
  text-align: center;
  padding: 2rem 1.5rem;
  opacity: 0;
  transform: translateY(1.5rem);
  transition: opacity 0.5s ease var(--delay), transform 0.5s ease var(--delay);
}

.step-card.visible {
  opacity: 1;
  transform: translateY(0);
}

.step-number {
  font-size: 0.8125rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: var(--color-violet-500);
  margin-bottom: 0.75rem;
}

.step-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 3.5rem;
  height: 3.5rem;
  border-radius: 1rem;
  margin-bottom: 1rem;
  background: var(--color-violet-100);
  color: var(--color-violet-600);
  transition: transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
}

:root.dark .step-icon {
  background: var(--color-violet-950);
  color: var(--color-violet-400);
}

.step-card:hover .step-icon {
  transform: scale(1.1) translateY(-2px);
}

.step-title {
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--color-neutral-900);
  margin-bottom: 0.5rem;
}

:root.dark .step-title {
  color: var(--color-neutral-100);
}

.step-description {
  font-size: 0.875rem;
  line-height: 1.6;
  color: var(--color-neutral-500);
}



/* ═══════════════════════════════════════════
   COMPARISON SECTION
   ═══════════════════════════════════════════ */
.comparison-section {
  padding: 5rem 0;
}

/* ═══════════════════════════════════════════
   FAQ SECTION
   ═══════════════════════════════════════════ */
.faq-section {
  padding: 5rem 0;
  background: var(--color-neutral-50);
  border-top: 1px solid var(--color-neutral-200);
  border-bottom: 1px solid var(--color-neutral-200);
}

:root.dark .faq-section {
  background: var(--color-neutral-950);
  border-color: var(--color-neutral-800);
}

/* ═══════════════════════════════════════════
   CTA SECTION
   ═══════════════════════════════════════════ */
.cta-section {
  position: relative;
  padding: 5rem 0;
  overflow: hidden;
}

.cta-bg {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.cta-gradient {
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse 80% 60% at 50% 110%, rgba(139, 92, 246, 0.1), transparent 70%);
}

:root.dark .cta-gradient {
  background: radial-gradient(ellipse 80% 60% at 50% 110%, rgba(139, 92, 246, 0.15), transparent 70%);
}

.cta-content {
  text-align: center;
  opacity: 0;
  transform: translateY(1.5rem);
  transition: opacity 0.6s ease, transform 0.6s ease;
}

.cta-content.visible {
  opacity: 1;
  transform: translateY(0);
}

.cta-title {
  font-size: clamp(1.75rem, 4vw, 2.25rem);
  font-weight: 700;
  letter-spacing: -0.025em;
  color: var(--color-neutral-900);
  margin-bottom: 0.75rem;
}

:root.dark .cta-title {
  color: var(--color-neutral-50);
}

.cta-description {
  font-size: 1.0625rem;
  color: var(--color-neutral-500);
  max-width: 28rem;
  margin: 0 auto 2rem;
  line-height: 1.6;
}

.cta-actions {
  display: flex;
  gap: 0.75rem;
  justify-content: center;
  flex-wrap: wrap;
}
</style>
