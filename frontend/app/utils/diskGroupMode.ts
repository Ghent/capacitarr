/**
 * Disk group mode visual treatment — single source of truth.
 *
 * Provides icon, color classes (for active pills and inline badges), and
 * tooltip i18n keys for the four disk-group operating modes:
 *   - auto      → red    (destructive, deletes automatically)
 *   - approval  → neutral (waits for human approval)
 *   - sunset    → amber  (winds down, reduces target percentage)
 *   - dry-run   → muted  (passive, no actions taken)
 *
 * Used by `DiskGroupSection.vue` (badge on dashboard cards) and
 * `RuleDiskThresholds.vue` (mode-selector pills in the rules editor).
 */
import { HandIcon, HourglassIcon, ShieldIcon, ZapIcon } from 'lucide-vue-next';
import type { Component } from 'vue';
import { MODE_APPROVAL, MODE_AUTO, MODE_SUNSET } from '~/constants';

/** Icon component for a given mode. */
export function modeIcon(mode: string): Component {
  switch (mode) {
    case MODE_AUTO:
      return ZapIcon;
    case MODE_APPROVAL:
      return HandIcon;
    case MODE_SUNSET:
      return HourglassIcon;
    default:
      return ShieldIcon;
  }
}

/**
 * Active-pill color classes — solid backgrounds for the rules editor pills.
 * Use when the mode is the *selected* state of a button group.
 */
export function modeActivePillClasses(mode: string): string {
  switch (mode) {
    case MODE_AUTO:
      return 'bg-red-600 hover:bg-red-700 text-white border-red-600';
    case MODE_APPROVAL:
      return '';
    case MODE_SUNSET:
      return 'bg-amber-600 hover:bg-amber-700 text-white border-amber-600';
    default:
      return 'bg-muted text-foreground hover:bg-muted/80';
  }
}

/**
 * Inline-badge color classes — translucent backgrounds for small status
 * badges that sit alongside text (e.g., dashboard disk-group cards).
 *
 * Use with `<UiBadge variant="outline">` so the border picks up the color
 * and the text/bg utilities take effect.
 */
export function modeBadgeClasses(mode: string): string {
  switch (mode) {
    case MODE_AUTO:
      return 'bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/30';
    case MODE_APPROVAL:
      return 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/30';
    case MODE_SUNSET:
      return 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/30';
    default:
      return 'bg-muted text-muted-foreground border-border';
  }
}

/** i18n key for the tooltip describing what each mode does. */
export function modeTooltipKey(mode: string): string {
  switch (mode) {
    case MODE_AUTO:
      return 'mode.autoTooltip';
    case MODE_APPROVAL:
      return 'mode.approvalTooltip';
    case MODE_SUNSET:
      return 'mode.sunsetTooltip';
    default:
      return 'mode.dryRunTooltip';
  }
}
