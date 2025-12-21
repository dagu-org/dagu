/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./src/**/*.{ts,tsx}'],
  theme: {
    container: {
      center: true,
      padding: '2rem',
      screens: {
        '2xl': '1400px',
      },
    },
    extend: {
      colors: {
        /* Core semantic colors */
        text: 'rgb(var(--text))',
        'text-secondary': 'rgba(var(--text-secondary), var(--text-secondary-alpha))',
        muted: 'rgba(var(--muted), var(--muted-alpha))',
        trace: 'rgba(var(--trace), var(--trace-alpha))',

        /* Surfaces */
        bg: 'rgb(var(--bg))',
        surface: 'rgb(var(--surface))',
        'accent-surface': 'rgb(var(--accent-surface))',
        'glass-bg': 'rgba(var(--glass-bg), var(--glass-bg-alpha))',
        'glass-border': 'rgb(var(--glass-border))',

        /* Borders */
        border: 'rgb(var(--border))',
        'border-bold': 'rgb(var(--border-bold))',

        /* Primary intent */
        primary: {
          DEFAULT: 'rgb(var(--primary))',
          highlight: 'rgba(var(--primary-highlight), var(--primary-highlight-alpha))',
          'alt-highlight': 'rgb(var(--primary-alt-highlight))',
          foreground: 'rgb(var(--primary-foreground))',
        },
        link: 'rgb(var(--link))',

        /* Secondary intent */
        secondary: {
          DEFAULT: 'rgba(var(--secondary), var(--secondary-alpha))',
          hover: 'rgb(var(--secondary-hover))',
          foreground: 'rgb(var(--secondary-foreground))',
        },

        /* Button component tokens */
        'button-primary-bg': 'rgb(var(--button-primary-bg))',
        'button-primary-border': 'rgb(var(--button-primary-border))',
        'button-primary-border-hover': 'rgb(var(--button-primary-border-hover))',
        'frame-primary-bg': 'rgb(var(--frame-primary-bg))',
        'frame-secondary-bg': 'rgb(var(--frame-secondary-bg))',
        'button-secondary-bg': 'rgb(var(--button-secondary-bg))',
        'button-secondary-border': 'rgb(var(--button-secondary-border))',
        'button-secondary-border-hover': 'rgb(var(--button-secondary-border-hover))',
        'frame-danger-bg': 'rgba(var(--frame-danger-bg), var(--frame-danger-bg-alpha))',
        'button-danger-border': 'rgb(var(--button-danger-border))',
        'button-danger-border-hover': 'rgb(var(--button-danger-border-hover))',

        /* Status: Danger */
        danger: {
          DEFAULT: 'rgb(var(--danger))',
          light: 'rgb(var(--danger-light))',
          lighter: 'rgb(var(--danger-lighter))',
          dark: 'rgb(var(--danger-dark))',
          highlight: 'rgba(var(--danger-highlight), var(--danger-highlight-alpha))',
        },

        /* Status: Warning */
        warning: {
          DEFAULT: 'rgb(var(--warning))',
          dark: 'rgb(var(--warning-dark))',
          highlight: 'rgba(var(--warning-highlight), var(--warning-highlight-alpha))',
        },

        /* Status: Success */
        success: {
          DEFAULT: 'rgb(var(--success))',
          light: 'rgb(var(--success-light))',
          dark: 'rgb(var(--success-dark))',
          highlight: 'rgba(var(--success-highlight), var(--success-highlight-alpha))',
        },

        /* Status: Info */
        info: {
          DEFAULT: 'rgb(var(--info))',
          light: 'rgb(var(--info-light))',
          highlight: 'rgba(var(--info-highlight), var(--info-highlight-alpha))',
        },

        /* Emphasis and selection */
        emphasis: 'rgb(var(--emphasis))',
        selection: 'rgba(var(--selection), var(--selection-alpha))',

        /* Code */
        'code-bg': 'rgb(var(--code-bg))',
        'code-text': 'rgb(var(--code-text))',

        /* Charts / Data viz */
        'chart-1': 'rgb(var(--chart-1))',
        'chart-2': 'rgb(var(--chart-2))',
        'chart-3': 'rgb(var(--chart-3))',
        'chart-4': 'rgb(var(--chart-4))',
        'chart-5': 'rgb(var(--chart-5))',
        'chart-6': 'rgb(var(--chart-6))',
        'chart-7': 'rgb(var(--chart-7))',
        'chart-8': 'rgb(var(--chart-8))',

        /* SHADCN COMPATIBILITY */
        background: 'var(--background)',
        foreground: 'var(--foreground)',
        card: {
          DEFAULT: 'var(--card)',
          foreground: 'var(--card-foreground)',
        },
        popover: {
          DEFAULT: 'var(--popover)',
          foreground: 'var(--popover-foreground)',
        },
        destructive: {
          DEFAULT: 'var(--destructive)',
          foreground: 'rgb(var(--destructive-foreground))',
        },
        accent: {
          DEFAULT: 'var(--accent)',
          foreground: 'var(--accent-foreground)',
        },
        input: 'var(--input)',
        ring: 'rgb(var(--ring))',

        /* Sidebar */
        sidebar: {
          DEFAULT: 'var(--sidebar)',
          foreground: 'var(--sidebar-foreground)',
          primary: 'var(--sidebar-primary)',
          'primary-foreground': 'var(--sidebar-primary-foreground)',
          accent: 'var(--sidebar-accent)',
          'accent-foreground': 'var(--sidebar-accent-foreground)',
          border: 'var(--sidebar-border)',
          ring: 'var(--sidebar-ring)',
        },
      },
      borderRadius: {
        lg: 'var(--radius)',
        md: 'calc(var(--radius) - 2px)',
        sm: 'calc(var(--radius) - 4px)',
      },
    },
  },
  plugins: [],
};
