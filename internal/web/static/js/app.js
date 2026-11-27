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
  // выбор пережил перезагрузку. Подпись называет действие, а не текущее
  // состояние — так понятнее, что произойдёт по нажатию.
  function themeLabel() {
    var dark = document.documentElement.dataset.theme !== 'light';
    var el = document.querySelector('[data-theme-label]');
    if (el) el.textContent = dark ? 'Светлое оформление' : 'Тёмное оформление';
  }
  document.addEventListener('DOMContentLoaded', themeLabel);
  themeLabel();

  document.addEventListener('click', function (e) {
    var el = e.target.closest('[data-action="toggle-theme"]');
    if (!el) return;
    var root = document.documentElement;
    var next = root.dataset.theme === 'light' ? 'dark' : 'light';
    root.dataset.theme = next;
    localStorage.setItem('makeme-theme', next);
    themeLabel();
  });

  // <details> сам по себе не закрывается по клику снаружи и по Escape —
  // без этого меню аккаунта остаётся висеть поверх страницы.
  function closeAccount(except) {
    document.querySelectorAll('[data-account][open]').forEach(function (d) {
      if (d !== except) d.open = false;
    });
  }
  document.addEventListener('click', function (e) {
    closeAccount(e.target.closest('[data-account]'));
  });
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') closeAccount(null);
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
  //
  // Сначала запись из data-src, если она есть. Записи Free Dictionary API
  // массово отдают 404, поэтому при любой ошибке воспроизведения — и когда
  // записи нет вовсе — слово произносит синтез речи браузера по data-text.
  // Синтез работает локально и в CSP не упирается. «Не воспроизводится»
  // остаётся только на случай, когда нет ни записи, ни синтеза.
  var player = null;

  function cannotPlay(btn) {
    btn.textContent = 'Не воспроизводится';
  }

  function speak(btn) {
    var text = btn.dataset.text;
    var synth = window.speechSynthesis;
    if (!text || !synth || typeof window.SpeechSynthesisUtterance !== 'function') {
      cannotPlay(btn);
      return;
    }
    var u = new window.SpeechSynthesisUtterance(text);
    u.lang = 'en-US';
    var voices = synth.getVoices() || [];
    for (var i = 0; i < voices.length; i++) {
      if (voices[i].lang && voices[i].lang.replace('_', '-').indexOf('en-US') === 0) {
        u.voice = voices[i];
        break;
      }
    }
    u.onerror = function (ev) {
      // Прерывание своим же следующим нажатием — не ошибка.
      if (ev && (ev.error === 'interrupted' || ev.error === 'canceled')) return;
      cannotPlay(btn);
    };
    synth.cancel();
    synth.speak(u);
  }

  document.addEventListener('click', function (e) {
    var btn = e.target.closest('[data-action="play-audio"]');
    if (!btn) return;
    var src = btn.dataset.src;
    if (!src) {
      speak(btn);
      return;
    }
    if (!player) player = new Audio();
    player.src = src;
    var played = player.play();
    if (played && typeof played.catch === 'function') {
      played.catch(function () { speak(btn); });
    }
  });

  // Авто-переход в /words. Фидбек слова несёт data-autoadvance с паузой в
  // миллисекундах; по её истечении нажимаем «Дальше» сами — через htmx, тем же
  // путём, что и клик, поэтому окно LOCAL_QUIET_MS ниже отмечает и этот запрос.
  // Атрибута нет (карточка предлагает отложить слово или сообщает, что оно
  // выучено) — ждём кнопку. Фокус на «Дальше» в любом случае: Enter — сразу.
  //
  // Таймер один на страницу. Он снимается на любом новом запросе (человек
  // нажал сам) и перед срабатыванием проверяет, что фрагмент ещё в документе.
  var advanceTimer = null;
  function clearAdvance() {
    if (advanceTimer) {
      clearTimeout(advanceTimer);
      advanceTimer = null;
    }
  }
  function armAdvance(root) {
    if (!root || !root.querySelector) return;
    var next = root.querySelector('[data-advance-next]');
    if (!next) return;
    clearAdvance();
    next.focus({ preventScroll: true });
    var holder = root.matches('[data-autoadvance]') ? root : root.querySelector('[data-autoadvance]');
    if (!holder) return;
    var ms = parseInt(holder.dataset.autoadvance, 10);
    if (!(ms > 0)) return;
    advanceTimer = setTimeout(function () {
      advanceTimer = null;
      if (document.contains(holder) && document.contains(next)) next.click();
    }, ms);
  }
  document.body.addEventListener('htmx:beforeRequest', clearAdvance);
  document.body.addEventListener('htmx:afterSwap', function (e) {
    armAdvance(e.target);
  });

  // Событие «прогресс изменился» на странице с вводом только показывает
  // предложение обновить. Автоподмена вырвала бы слот из-под рук у человека,
  // который в этот момент печатает ответ.
  //
  // Своё же действие баннером не отмечаем: хаб не различает сессии и шлёт
  // событие всем вкладкам пользователя, включая ту, что и ответила. Тащить
  // идентификатор сокета в доменное событие ради этого неправильно, поэтому
  // подавляем на клиенте — окно после собственного запроса.
  // Засекаем на beforeRequest, а не на afterRequest: сервер публикует событие
  // внутри обработки запроса, поэтому сообщение из сокета успевает прийти
  // раньше, чем htmx отчитается о завершении.
  var LOCAL_QUIET_MS = 3000;
  var lastLocalChange = 0;
  document.body.addEventListener('htmx:beforeRequest', function () {
    lastLocalChange = Date.now();
  });
  document.body.addEventListener('htmx:afterRequest', function () {
    lastLocalChange = Date.now();
  });

  document.body.addEventListener('makeme:progress', function () {
    if (Date.now() - lastLocalChange < LOCAL_QUIET_MS) return;
    var note = document.querySelector('[data-live-note]');
    if (note) note.dataset.shown = '1';
  });
  document.addEventListener('click', function (e) {
    if (e.target.closest('[data-action="reload"]')) location.reload();
  });

  // Фокус в поле ввода после подмены слота, иначе печатать можно только
  // после клика мышью.
  document.body.addEventListener('htmx:afterSwap', function (e) {
    var field = e.target.querySelector('.field');
    if (field) field.focus();
  });
})();
