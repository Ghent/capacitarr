# Notifications

Capacitarr provides real-time notifications through Discord webhooks and Apprise (supporting 80+ notification services). Notifications keep you informed about engine activity, disk usage alerts, and system events without needing to check the dashboard. System events are also recorded in the **Activity Log** on the dashboard for at-a-glance visibility.

## Notification Types

### Cycle Digests

A cycle digest is a single summary notification sent after each engine run completes. It includes the full picture of what happened during the cycle — how many items were evaluated, how many were flagged, what was deleted, and how much space was freed.

Digest content varies by execution mode:

| Mode | Title | Summary |
|------|-------|---------|
| **Auto** | 🧹 Cleanup Complete | `Deleted X of Y evaluated items in Z.Zs, freeing N.N GB` |
| **Dry-run** | 🔍 Dry-Run Complete | `Flagged X of Y items in Z.Zs — Would free N.N GB` |
| **Approval** | 📋 Items Queued for Approval | `Queued X of Y items in Z.Zs — Potential N.N GB` |
| *(no action needed)* | ✅ All Clear | `Evaluated X items — no action needed` |

Auto-mode digests also include a disk usage progress bar showing the before/after usage percentage and target. If a newer Capacitarr version is available, a version banner is appended to the digest.

Digests use a **two-gate flush** internally — the notification waits until both the evaluation phase and the deletion phase of the engine run are complete before sending. This ensures deletion results (freed bytes, failure count) are included in the summary.

### Instant Alerts

Instant alerts fire immediately when their trigger event occurs — they are not batched or delayed. Each alert type covers a specific operational event:

| Alert Type | Description |
|------------|-------------|
| **Engine Error** | The evaluation engine encountered an error during a run |
| **Mode Changed** | The execution mode was switched (e.g., dry-run → auto) |
| **Server Started** | Capacitarr has started and is ready to accept requests |
| **Threshold Breached** | Disk usage has exceeded the configured threshold for a disk group |
| **Update Available** | A newer Capacitarr release was detected on GitLab |
| **Approval Activity** | An item was approved or rejected in the approval queue |
| **Integration Status** | An integration has failed its connection test or recovered from a previous failure |

## Discord Setup

### Step 1: Create a Webhook

1. Open your Discord server and navigate to the channel where you want notifications
2. Click the **gear icon** (⚙️) next to the channel name to open Channel Settings
3. Select **Integrations** from the sidebar
4. Click **Webhooks** → **New Webhook**
5. Give the webhook a name (e.g., "Capacitarr") and optionally set an avatar
6. Click **Copy Webhook URL** — you'll need this in the next step
7. Click **Save Changes**

### Step 2: Add Channel in Capacitarr

1. Navigate to **Settings** → **Notifications**
2. Click **Add Channel**
3. Select **Discord** as the channel type
4. Paste the webhook URL you copied from Discord
5. Give the channel a descriptive name (e.g., "Media Alerts")
6. Click **Save**

### Step 3: Configure Subscriptions

After saving the channel, configure which notifications it receives using the subscription toggles. Each toggle controls a specific category of events — see the [Subscription Toggles](#subscription-toggles) section below.

Use the **Test** button to verify the webhook is working. A test notification will appear in your Discord channel.

## Apprise Setup

[Apprise](https://github.com/caronc/apprise) is a self-hosted notification aggregator that supports 80+ notification services including Telegram, Matrix, Pushover, ntfy, Gotify, Email, Slack, Microsoft Teams, and many more. By configuring a single Apprise channel in Capacitarr, you can route notifications to any service Apprise supports.

### Step 1: Deploy an Apprise Server

Run Apprise API as a Docker container alongside Capacitarr:

```yaml
services:
  apprise:
    image: caronc/apprise:latest
    container_name: apprise
    ports:
      - "8000:8000"
    volumes:
      - apprise-config:/config
    restart: unless-stopped

volumes:
  apprise-config:
```

Once running, configure your notification URLs in the Apprise server. Refer to the [Apprise documentation](https://github.com/caronc/apprise/wiki) for supported services and URL formats.

### Step 2: Add Channel in Capacitarr

1. Navigate to **Settings** → **Notifications**
2. Click **Add Channel**
3. Select **Apprise** as the channel type
4. Enter the **Apprise Server URL** — this is the base URL of your Apprise API instance (e.g., `http://apprise:8000`)
5. Optionally enter **Tags** — a comma-separated list of Apprise tags to route the notification to specific destinations (e.g., `telegram,email`). If left empty, all configured notification URLs on the Apprise server receive the message.
6. Give the channel a descriptive name (e.g., "Telegram via Apprise")
7. Click **Save**

### Step 3: Configure Subscriptions

After saving the channel, configure which notifications it receives using the subscription toggles. Each toggle controls a specific category of events — see the [Subscription Toggles](#subscription-toggles) section below.

Use the **Test** button to verify the Apprise connection is working.

### Apprise URL Format

The Apprise Server URL should point to the root of your Apprise API instance. Capacitarr sends notifications to the `POST {url}/api/notify/` endpoint.

**Examples:**

| Network Setup | URL |
|---------------|-----|
| Same Docker network | `http://apprise:8000` |
| Different host | `http://192.168.1.100:8000` |
| Behind reverse proxy | `https://apprise.example.com` |

### Apprise Tags

Tags let you route notifications to specific destinations configured on your Apprise server. For example, if your Apprise server has notification URLs tagged with `urgent` and `info`, you can create two Capacitarr channels — one that sends to `urgent` (for threshold breaches and errors) and one that sends to `info` (for cycle digests).

If no tags are specified, the notification is sent to **all** notification URLs configured on the Apprise server.

## Subscription Toggles

Each external notification channel (Discord or Apprise) has independent subscription toggles. You can enable or disable each event category per channel, allowing you to route different notification types to different channels.

| Toggle | Events | Description |
|--------|--------|-------------|
| **Cycle Digest** | Engine complete + deletion batch | Summary of each engine run with stats and disk usage |
| **Error** | Engine errors | Fires when the evaluation engine fails during a run |
| **Mode Changed** | Execution mode switch | Fires when switching between dry-run, approval, and auto |
| **Server Started** | Application startup | Confirms Capacitarr is online after a restart |
| **Threshold Breach** | Disk usage exceeds threshold | Immediate alert when a disk group exceeds its threshold |
| **Update Available** | New version detected | Fires when a newer Capacitarr release exists on GitLab |
| **Approval Activity** | Items approved or rejected | Fires when approval queue items are approved or rejected |
| **Integration Status** | Integration failure or recovery | Fires when an integration fails its connection test or recovers |

## Digest Format

Cycle digest notifications are rendered as rich embeds in Discord and as Markdown messages for Apprise. Here's what a typical auto-mode digest looks like:

```
⚡ Capacitarr v2.0.0 • auto
─────────────────────────────
🧹 Cleanup Complete

Deleted 12 of 97 evaluated items
in 3.2s, freeing 48.3 GB

▓▓▓▓▓▓▓▓▓▓▓▓▓▓░░░░░░ 72% → 65%

📦 v2.1.0 available!
```

Digest components:

- **Author line:** Shows the Capacitarr version and current execution mode
- **Title:** Mode-specific title (🧹 Cleanup Complete, 🔍 Dry-Run Complete, 📋 Items Queued, or ✅ All Clear)
- **Description:** Item counts, duration, and freed/potential space
- **Progress bar:** Visual disk usage indicator (auto mode and all-clear only) showing current percentage and target
- **Version banner:** Appears when a newer release is available (optional)

Alert notifications use a similar format with a title, message, and color-coded severity (green for success, blue for info, amber for attention, red for errors).
