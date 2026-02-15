# Dark Mode Implementation Requirements Document

## 1. Executive Summary

This document outlines the requirements for implementing dark mode in the Boltbase UI. The implementation will provide a modern, developer-friendly dark theme as the default, with the ability to switch to light mode. The solution leverages existing CSS variable infrastructure and Tailwind CSS dark mode capabilities.

## 2. Business Requirements

### 2.1 Objectives
- Provide a minimal, modern, and information-dense dark UI that developers prefer
- Reduce eye strain for users working in low-light environments
- Maintain consistency with modern development tools and IDEs
- Enhance the professional appearance of the Boltbase workflow orchestration engine

### 2.2 Success Criteria
- Dark mode is active by default on first visit
- Theme preference persists across browser sessions
- All UI elements maintain proper contrast and readability
- Zero visual regressions in existing functionality
- Smooth, professional transitions between themes

## 3. Functional Requirements

### 3.1 Theme Management
- **FR-1**: System SHALL default to dark mode for new users
- **FR-2**: System SHALL persist theme preference in browser localStorage
- **FR-3**: System SHALL apply theme immediately without page reload
- **FR-4**: System SHALL respect user's theme preference on subsequent visits

### 3.2 Theme Toggle
- **FR-5**: System SHALL provide a theme toggle button in the main navigation
- **FR-6**: Toggle SHALL use intuitive sun/moon icons
- **FR-7**: Toggle SHALL provide visual feedback on hover and click
- **FR-8**: Toggle SHALL be accessible via keyboard navigation

### 3.3 Visual Consistency
- **FR-9**: All components SHALL respect the active theme
- **FR-10**: System SHALL maintain consistent color scheme across all pages
- **FR-11**: Charts and visualizations SHALL adapt to the active theme
- **FR-12**: Code syntax highlighting SHALL remain readable in both themes

## 4. Technical Requirements

### 4.1 Architecture
- **TR-1**: Implementation SHALL extend existing UserPreference context
- **TR-2**: Theme state SHALL be managed through React Context API
- **TR-3**: Dark mode SHALL be activated by adding 'dark' class to document root
- **TR-4**: Implementation SHALL use existing CSS variable system

### 4.2 Storage
- **TR-5**: Theme preference SHALL be stored with key 'dagu-theme'
- **TR-6**: Storage SHALL use values: 'dark' | 'light'
- **TR-7**: System SHALL handle missing/corrupted storage gracefully

### 4.3 Performance
- **TR-8**: Theme switching SHALL complete within 100ms
- **TR-9**: Theme application SHALL not cause layout shifts
- **TR-10**: Initial theme detection SHALL not delay page render

### 4.4 Browser Compatibility
- **TR-11**: Solution SHALL work in all modern browsers (Chrome, Firefox, Safari, Edge)
- **TR-12**: Solution SHALL gracefully degrade in older browsers

## 5. Design Requirements

### 5.1 Color System
Dark mode colors (already defined in CSS):
```css
--background: oklch(0.145 0 0);      /* Near black */
--foreground: oklch(0.985 0 0);      /* Near white */
--card: oklch(0.145 0 0);            /* Same as background */
--primary: #222222;                   /* Dark gray */
--secondary: oklch(0.269 0 0);        /* Darker gray */
--muted: oklch(0.269 0 0);           /* Muted elements */
--destructive: oklch(0.396 0.141 25.723); /* Error red */
--border: oklch(0.269 0 0);          /* Border color */
```

### 5.2 Component Updates
Priority components requiring color updates:
1. Layout.tsx - Main application layout
2. Navigation components - Sidebar and navbar
3. Table components - Data grids and lists
4. Modal dialogs - All overlay components
5. Form elements - Inputs, selects, buttons
6. Status indicators - Badges and chips

### 5.3 Hardcoded Color Replacements
| Current | Replacement |
|---------|-------------|
| `bg-white` | `bg-background` |
| `bg-gray-50` | `bg-muted` |
| `bg-gray-100` | `bg-muted` |
| `bg-gray-200` | `bg-border` |
| `text-black` | `text-foreground` |
| `text-gray-*` | `text-muted-foreground` |
| `border-gray-*` | `border-border` |

## 6. Implementation Plan

### 6.1 Phase 1: Core Infrastructure (Priority: High)
1. Extend UserPreference context with theme management
2. Add theme initialization logic (default to dark)
3. Implement localStorage persistence
4. Add document root class manipulation

### 6.2 Phase 2: UI Integration (Priority: High)
1. Create theme toggle component
2. Add toggle to navigation bar
3. Implement smooth transitions
4. Update Layout.tsx component

### 6.3 Phase 3: Component Migration (Priority: Medium)
1. Audit all components for hardcoded colors
2. Replace with theme-aware classes
3. Test each component in both themes
4. Fix contrast and visibility issues

### 6.4 Phase 4: Testing & Polish (Priority: Low)
1. Cross-browser testing
2. Accessibility testing
3. Performance optimization
4. Documentation updates

## 7. Testing Requirements

### 7.1 Unit Tests
- Theme context functionality
- localStorage persistence
- Theme toggle component
- Default theme initialization

### 7.2 Integration Tests
- Theme switching across routes
- Persistence across sessions
- Component theme compliance

### 7.3 Visual Tests
- Screenshot comparison for all major views
- Contrast ratio validation
- Color consistency checks

### 7.4 Manual Test Cases
1. First-time user sees dark mode
2. Theme preference persists after browser restart
3. All pages render correctly in both themes
4. No flash of wrong theme on page load
5. Theme toggle is accessible via keyboard

## 8. Acceptance Criteria

- [ ] Dark mode is default for new users
- [ ] Theme toggle is visible and functional
- [ ] All hardcoded colors are replaced
- [ ] Theme preference persists
- [ ] No visual regressions
- [ ] All components respect active theme
- [ ] Smooth transitions between themes
- [ ] Passes accessibility standards

## 9. Risks and Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Flash of wrong theme | High | Implement blocking script to apply theme before render |
| Missed hardcoded colors | Medium | Automated CSS audit script |
| Browser incompatibility | Low | Progressive enhancement approach |
| Performance impact | Low | Minimize reflows, use CSS transitions |

## 10. Future Enhancements

- System theme detection (prefers-color-scheme)
- Additional theme options (high contrast, etc.)
- Theme customization capabilities
- Automatic theme switching based on time of day

## 11. References

- Existing CSS variables: `/ui/src/styles/global.css`
- Tailwind configuration: `/ui/tailwind.config.js`
- UserPreference context: `/ui/src/contexts/UserPreference.tsx`
- Layout component: `/ui/src/layouts/Layout.tsx`

---

**Document Version**: 1.0  
**Date**: January 2025  
**Status**: Draft