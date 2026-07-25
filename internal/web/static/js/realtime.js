// Живые обновления. Сокет несёт только имя события — что именно перерисовать,
// решает страница, потому что сервер не знает, на какой из них открыта вкладка.
//
// Событие превращается в DOM-событие на <body>, и элементы подписываются на
// него через hx-trigger="makeme:leaderboard from:body". Так обновление идёт по
// тому же пути, что и обычная загрузка, — один код рендеринга на оба случая.
(function () {
  'use strict';

  var RETRY_MIN = 1000;
  var RETRY_MAX = 30000;
  var retry = RETRY_MIN;
  var socket = null;

  function url() {
    var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    return proto + '//' + location.host + '/ws';
  }

  function connect() {
    try {
      socket = new WebSocket(url());
    } catch (e) {
      schedule();
      return;
    }

    socket.addEventListener('open', function () {
      retry = RETRY_MIN;
    });

    socket.addEventListener('message', function (e) {
      document.body.dispatchEvent(
        new CustomEvent('makeme:' + e.data, { bubbles: true })
      );
    });

    socket.addEventListener('close', schedule);
    // 'error' is always followed by 'close', so reconnecting is handled there.
  }

  // Экспоненциальная пауза с потолком: сервер мог уйти на деплой, и долбить
  // его раз в секунду бессмысленно.
  function schedule() {
    socket = null;
    setTimeout(connect, retry);
    retry = Math.min(retry * 2, RETRY_MAX);
  }

  // Вкладка в фоне ничего не показывает — соединение всё равно держим, чтобы
  // при возврате состояние было свежим, но при пробуждении проверяем связь.
  document.addEventListener('visibilitychange', function () {
    if (!document.hidden && !socket) connect();
  });

  connect();
})();
