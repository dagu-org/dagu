/**
 * Slugify a heading string into a URL-safe anchor ID.
 * Used by both the markdown preview (to set heading IDs) and the outline panel
 * (to generate matching anchors). Must stay in sync across both call sites.
 */
export function slugifyHeading(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-');
}

/**
 * Get responsive text size class based on string length.
 * Used for titles in headers and sidebars to ensure they fit properly.
 *
 * @param text - The text to measure
 * @param variant - The context where the text will be displayed
 * @returns Tailwind CSS text size class
 */
export function getResponsiveTitleClass(
  text: string,
  variant: 'sidebar-expanded' | 'sidebar-mobile' | 'header-mobile' = 'sidebar-expanded'
): string {
  const length = text.length;

  switch (variant) {
    case 'sidebar-expanded':
      // Desktop sidebar (menu.tsx)
      if (length > 20) return 'text-xs';
      if (length > 15) return 'text-sm';
      if (length > 10) return 'text-base';
      return 'text-lg';

    case 'sidebar-mobile':
      // Mobile sidebar (Layout.tsx)
      if (length > 20) return 'text-sm';
      if (length > 15) return 'text-base';
      return 'text-lg';

    case 'header-mobile':
      // Mobile header (Layout.tsx)
      if (length > 20) return 'text-xs';
      if (length > 15) return 'text-xs';
      return 'text-xs';

    default:
      return 'text-base';
  }
}
