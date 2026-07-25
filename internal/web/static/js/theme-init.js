// Применяет сохранённую тему до первой отрисовки, иначе тёмная тема мигает
// светлым. Подключается без defer — это единственный скрипт, которому
// действительно нужно блокировать рендер.
//
// Живёт отдельным файлом, а не инлайном, чтобы CSP оставался script-src 'self'
// без 'unsafe-inline'.
(function () {
  var t = localStorage.getItem('makeme-theme');
  if (t) document.documentElement.dataset.theme = t;
})();
