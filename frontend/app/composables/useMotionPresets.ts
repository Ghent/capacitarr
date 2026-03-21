/**
 * Shared v-motion animation presets.
 *
 * Provides consistent spring-physics entrance animations across all components.
 * Use these presets with v-motion's `:initial` / `:enter` / `:leave` props to
 * avoid repeating the same stiffness/damping values in every component.
 *
 * @example
 * ```vue
 * <script setup lang="ts">
 * const { cardEntrance } = useMotionPresets();
 * </script>
 * <template>
 *   <UiCard v-motion v-bind="cardEntrance">...</UiCard>
 * </template>
 * ```
 */
export function useMotionPresets() {
  const spring = { type: 'spring' as const, stiffness: 260, damping: 24 };

  /** Standard card entrance: fade-in + slide up 12px. */
  const cardEntrance = {
    initial: { opacity: 0, y: 12 },
    enter: { opacity: 1, y: 0, transition: spring },
  };

  /** Simple opacity fade. */
  const fadeIn = {
    initial: { opacity: 0 },
    enter: { opacity: 1, transition: spring },
  };

  /** Scale + fade entrance (for modals, popovers, hero elements). */
  const scaleIn = {
    initial: { opacity: 0, scale: 0.96, y: 10 },
    enter: { opacity: 1, scale: 1, y: 0, transition: spring },
  };

  /** Slide from left + fade. */
  const slideInLeft = {
    initial: { opacity: 0, x: -8 },
    enter: { opacity: 1, x: 0, transition: spring },
  };

  /** Slide from right + fade. */
  const slideInRight = {
    initial: { opacity: 0, x: 8 },
    enter: { opacity: 1, x: 0, transition: spring },
  };

  /** Slide up from bottom (for toolbars, footers). */
  const slideUpFromBottom = {
    initial: { opacity: 0, y: 16 },
    enter: { opacity: 1, y: 0, transition: spring },
  };

  /** Banner slide in from top. */
  const slideDownFromTop = {
    initial: { opacity: 0, y: -8 },
    enter: { opacity: 1, y: 0, transition: spring },
  };

  /**
   * Per-item list entrance with optional stagger delay.
   *
   * @param delay - Delay in ms before the animation starts (e.g., `index * 30`)
   */
  function listItem(delay = 0) {
    return {
      initial: { opacity: 0, x: -8 },
      enter: {
        opacity: 1,
        x: 0,
        transition: { ...spring, delay },
      },
      leave: { opacity: 0, x: 8 },
    };
  }

  /**
   * Staggered grid item entrance with scale + fade.
   *
   * @param delay - Delay in ms (e.g., `index * 30`, capped at 300)
   */
  function gridItem(delay = 0) {
    return {
      initial: { opacity: 0, scale: 0.95 },
      enter: {
        opacity: 1,
        scale: 1,
        transition: { ...spring, delay: Math.min(delay, 300) },
      },
    };
  }

  return {
    cardEntrance,
    fadeIn,
    scaleIn,
    slideInLeft,
    slideInRight,
    slideUpFromBottom,
    slideDownFromTop,
    listItem,
    gridItem,
  };
}
