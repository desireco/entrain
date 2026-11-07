
(function () {
  'use strict';
  var $ = function (s) { return document.querySelector(s); };
  var reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  var fitCanvas = window.entrainFitCanvas;
  /* seamless ticker: duplicate the sequence */
  var track = document.getElementById('ticker-track');
  if (track) track.appendChild(track.querySelector('.ticker-seq').cloneNode(true));

  /* ============ Hero canvas: drifting sine field ============ */
  var hero = $('#hero-canvas'), hctx = hero.getContext('2d');
  var t0 = performance.now();
  function drawHero(now) {
    var dpr = fitCanvas(hero);
    if (!dpr) { requestAnimationFrame(drawHero); return; }
    var W = hero.width, H = hero.height, T = (now - t0) / 1000;
    hctx.clearRect(0, 0, W, H);
    var layers = [
      { amp: 0.16, k: 1.6, sp: 0.11, col: 'rgba(194,65,12,0.26)', lw: 1.6 },
      { amp: 0.10, k: 2.6, sp: 0.17, col: 'rgba(194,65,12,0.16)', lw: 1.2 },
      { amp: 0.07, k: 4.1, sp: 0.23, col: 'rgba(63,59,50,0.10)', lw: 1 }
    ];
    for (var L = 0; L < layers.length; L++) {
      var ly = layers[L];
      hctx.beginPath();
      for (var x = 0; x <= W; x += 6) {
        var p = x / W;
        var y = H * (0.62 + L * 0.07)
          + Math.sin(p * Math.PI * 2 * ly.k + T * ly.sp * 2 * Math.PI + L * 1.7) * H * ly.amp
          + Math.sin(p * Math.PI * 2 * (ly.k * 0.5) - T * ly.sp * 1.3 * Math.PI) * H * ly.amp * 0.5;
        x === 0 ? hctx.moveTo(x, y) : hctx.lineTo(x, y);
      }
      hctx.strokeStyle = ly.col; hctx.lineWidth = ly.lw * dpr; hctx.stroke();
    }
    if (!reducedMotion) requestAnimationFrame(drawHero);
  }
  requestAnimationFrame(drawHero);
  window.addEventListener('resize', function () { if (reducedMotion) { drawHero(performance.now()); } });

})();
