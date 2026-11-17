// 深浅主题：默认跟随系统，可手动切换并记住
(function () {
  var root = document.documentElement;
  var KEY = 'lslan-theme';

  var stored = null;
  try { stored = localStorage.getItem(KEY); } catch (e) {}

  function apply(mode) {
    if (mode === 'light' || mode === 'dark') {
      root.setAttribute('data-theme', mode);
    } else {
      root.removeAttribute('data-theme');
    }
  }

  var mode = stored;
  if (mode !== 'light' && mode !== 'dark') mode = null;
  apply(mode);

  var btn = document.getElementById('themeToggle');
  if (btn) {
    btn.addEventListener('click', function () {
      var systemDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      var current = root.getAttribute('data-theme') || (systemDark ? 'dark' : 'light');
      var next = current === 'dark' ? 'light' : 'dark';
      apply(next);
      try { localStorage.setItem(KEY, next); } catch (e) {}
    });
  }
})();
