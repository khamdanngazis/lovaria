// JS landing page (T18): font Google dimuat tanpa memblokir render (preload →
// stylesheet) tanpa atribut onload inline (CSP).
document.querySelectorAll('link[data-font-css]').forEach(function (l) {
  l.rel = 'stylesheet';
});
