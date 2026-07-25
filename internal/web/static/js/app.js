// Клиентские мелочи веб-интерфейса.
//
// CSP здесь — script-src 'self' без 'unsafe-inline'. Это ограничивает разметку,
// и ограничение намеренное. Прежде чем добавлять что-то в шаблоны:
//   — никаких инлайновых <script>;
//   — никаких обработчиков в атрибутах (onclick="…" и прочие on*);
//   — никаких htmx-выражений hx-on / js:.
// Вместо этого — data-атрибут в разметке и обработчик здесь.

(function () {
  'use strict';

  // Переключатель темы: [data-theme] на <html> плюс localStorage, чтобы
  // выбор пережил перезагрузку.
  document.addEventListener('click', function (e) {
    var el = e.target.closest('[data-action="toggle-theme"]');
    if (!el) return;
    var root = document.documentElement;
    var next = root.dataset.theme === 'light' ? 'dark' : 'light';
    root.dataset.theme = next;
    localStorage.setItem('makeme-theme', next);
  });

  // Мгновенная отметка выбранного варианта: ответ уходит на сервер и вернётся
  // фрагментом, но без этого клик ощущается залипшим на время round-trip.
  document.addEventListener('click', function (e) {
    var opt = e.target.closest('.opt');
    if (!opt || opt.disabled) return;
    opt.setAttribute('aria-pressed', 'true');
  });

  // Произношение. Один переиспользуемый <audio> вместо элемента на карточку —
  // карточки приезжают фрагментами и умирают, аудио должно их пережить.
  var player = null;
  document.addEventListener('click', function (e) {
    var btn = e.target.closest('[data-action="play-audio"]');
    if (!btn) return;
    var src = btn.dataset.src;
    if (!src) return;
    if (!player) player = new Audio();
    player.src = src;
    player.play().catch(function () {
      btn.textContent = 'Не воспроизводится';
    });
  });

  // Фокус в поле ввода после подмены слота, иначе печатать можно только
  // после клика мышью.
  document.body.addEventListener('htmx:afterSwap', function (e) {
    var field = e.target.querySelector('.field');
    if (field) field.focus();
  });
})();
