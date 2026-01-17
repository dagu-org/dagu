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
      // Theme is now primarily defined in src/styles/global.css via @theme
      // This config remains for content paths and container settings
    },
  },
  plugins: [],
};
