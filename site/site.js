window.entrainFitCanvas = function(cv) {
  var dpr = Math.min(window.devicePixelRatio || 1, 2);
  var r = cv.getBoundingClientRect();
  if (r.width === 0) return null;
  var w = Math.round(r.width * dpr), h = Math.round(r.height * dpr);
  if (cv.width !== w || cv.height !== h) { cv.width = w; cv.height = h; }
  return dpr;
};

const reveal = document.querySelectorAll('.reveal');
if ('IntersectionObserver' in window) {
  document.documentElement.classList.add('observe-reveals');
  const observer = new IntersectionObserver(entries => {
    for (const entry of entries) {
      if (entry.isIntersecting) {
        entry.target.classList.add('in');
        observer.unobserve(entry.target);
      }
    }
  });
  reveal.forEach(element => observer.observe(element));
} else {
  reveal.forEach(element => element.classList.add('in'));
}
