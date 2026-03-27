declare module 'virtual:announcements' {
  export interface Announcement {
    id: string;
    title: string;
    type: 'info' | 'warning' | 'critical';
    date: string;
    expires?: string;
    active: boolean;
    /** Pre-rendered HTML from the markdown body. */
    body: string;
  }

  const announcements: Announcement[];
  export default announcements;
}
