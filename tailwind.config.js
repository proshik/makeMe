/** @type {import('tailwindcss').Config} */
module.exports = {
  // Только шаблоны веб-адаптера: по ним CLI и решает, какие утилиты попадут
  // в сборку. Классы, собранные в Go строками, сюда не попадут — если такие
  // появятся, их нужно перечислить в safelist.
  content: ['./internal/web/templates/**/*.html'],
  theme: {
    // Обнуляем полностью, а не через extend: иначе `rounded-lg` и `shadow-md`
    // остаются доступными и рано или поздно окажутся в разметке. Дизайн-система
    // «Табло» запрещает скругления, тени и градиенты.
    borderRadius: { none: '0px' },
    boxShadow: { none: 'none' },
    extend: {
      fontFamily: {
        display: ['Oswald', 'sans-serif'],
        sans: ['"IBM Plex Sans"', 'sans-serif'],
        mono: ['"IBM Plex Mono"', 'monospace'],
      },
    },
  },
  corePlugins: {
    // Палитра и темы живут в CSS-переменных в app.css, а не в теме Tailwind,
    // поэтому цветовые и градиентные утилиты не нужны вовсе.
    gradientColorStops: false,
    backgroundImage: false,
  },
  plugins: [],
}
