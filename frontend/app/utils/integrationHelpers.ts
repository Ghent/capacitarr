import {
  TvIcon,
  FilmIcon,
  PlayCircleIcon,
  ServerIcon,
  ActivityIcon,
  InboxIcon,
  MusicIcon,
  BookOpenIcon,
  MonitorPlayIcon,
} from 'lucide-vue-next';
import type { FunctionalComponent } from 'vue';

export function typeIcon(type: string): FunctionalComponent {
  switch (type) {
    case 'sonarr':
      return TvIcon;
    case 'radarr':
      return FilmIcon;
    case 'lidarr':
      return MusicIcon;
    case 'readarr':
      return BookOpenIcon;
    case 'plex':
      return PlayCircleIcon;
    case 'jellyfin':
      return MonitorPlayIcon;
    case 'emby':
      return MonitorPlayIcon;
    case 'tautulli':
      return ActivityIcon;
    case 'overseerr':
      return InboxIcon;
    default:
      return ServerIcon;
  }
}

export function typeColor(type: string): string {
  switch (type) {
    case 'sonarr':
      return 'bg-sky-500';
    case 'radarr':
      return 'bg-amber-500';
    case 'lidarr':
      return 'bg-green-500';
    case 'readarr':
      return 'bg-emerald-600';
    case 'plex':
      return 'bg-orange-500';
    case 'jellyfin':
      return 'bg-purple-500';
    case 'emby':
      return 'bg-emerald-500';
    case 'tautulli':
      return 'bg-teal-500';
    case 'overseerr':
      return 'bg-indigo-500';
    default:
      return 'bg-muted-foreground';
  }
}

export function typeTextColor(type: string): string {
  switch (type) {
    case 'sonarr':
      return 'text-sky-500';
    case 'radarr':
      return 'text-amber-500';
    case 'lidarr':
      return 'text-green-500';
    case 'readarr':
      return 'text-emerald-600';
    case 'plex':
      return 'text-orange-500';
    case 'jellyfin':
      return 'text-purple-500';
    case 'emby':
      return 'text-emerald-500';
    case 'tautulli':
      return 'text-teal-500';
    case 'overseerr':
      return 'text-indigo-500';
    default:
      return 'text-muted-foreground';
  }
}

export const namePlaceholders: Record<string, string> = {
  sonarr: 'My Sonarr',
  radarr: 'My Radarr',
  lidarr: 'My Lidarr',
  readarr: 'My Readarr',
  plex: 'My Plex',
  jellyfin: 'My Jellyfin',
  emby: 'My Emby',
  tautulli: 'My Tautulli',
  overseerr: 'My Overseerr',
};

export const urlPlaceholders: Record<string, string> = {
  sonarr: 'http://localhost:8989',
  radarr: 'http://localhost:7878',
  lidarr: 'http://localhost:8686',
  readarr: 'http://localhost:8787',
  plex: 'http://192.168.1.100:32400',
  jellyfin: 'http://localhost:8096',
  emby: 'http://localhost:8096',
  tautulli: 'http://localhost:8181',
  overseerr: 'http://localhost:5055',
};

export const urlHelpTexts: Record<string, string> = {
  sonarr: 'Your Sonarr instance URL (IP or hostname + port).',
  radarr: 'Your Radarr instance URL (IP or hostname + port).',
  lidarr: 'Your Lidarr instance URL (IP or hostname + port).',
  readarr: 'Your Readarr instance URL (IP or hostname + port).',
  plex: 'Your Plex Media Server URL. Use the direct server address, not app.plex.tv.',
  jellyfin: 'Your Jellyfin server URL (IP or hostname + port).',
  emby: 'Your Emby server URL (IP or hostname + port).',
  tautulli: 'Your Tautulli instance URL (IP or hostname + port).',
  overseerr: 'Full URL including any subpath (e.g., https://example.com/requests/).',
};
