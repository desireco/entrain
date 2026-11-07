(function () {
  'use strict';
  var $ = (s) => document.querySelector(s);
  /* ============ Handbook tabs ============ */
  var tabs = document.querySelectorAll('.tab-btn');
  var tabNames = { install: 'install', render: 'render', verify: 'verify', noise: 'noise' };
  function selectTab(btn) {
    tabs.forEach(function (b) {
      var on = b === btn;
      b.setAttribute('aria-selected', String(on));
      document.getElementById(b.getAttribute('aria-controls')).hidden = !on;
    });
    $('#term-tab-name').textContent = 'entrain · ' + tabNames[btn.getAttribute('aria-controls').replace('panel-', '')];
  }
  tabs.forEach(function (b, idx) {
    b.addEventListener('click', function () { selectTab(b); });
    b.addEventListener('keydown', function (e) {
      if (e.key === 'ArrowDown' || e.key === 'ArrowRight') { tabs[(idx + 1) % tabs.length].focus(); }
      if (e.key === 'ArrowUp' || e.key === 'ArrowLeft') { tabs[(idx - 1 + tabs.length) % tabs.length].focus(); }
    });
  });

})();
