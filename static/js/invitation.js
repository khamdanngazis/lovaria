// JS halaman undangan (tanpa library): lightbox galeri + fade-in saat scroll.
(function () {
  // Fade-in bagian halaman saat masuk layar (dilewati bila pengguna memilih
  // mengurangi animasi).
  var reduce = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (!reduce && 'IntersectionObserver' in window) {
    var sections = document.querySelectorAll('main > section:not(#opening)');
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (e) {
        if (e.isIntersecting) {
          e.target.classList.add('lv-shown');
          io.unobserve(e.target);
        }
      });
    }, { rootMargin: '0px 0px -10% 0px' });
    sections.forEach(function (s) {
      s.classList.add('lv-reveal');
      io.observe(s);
    });
  }

  // Lightbox: klik foto galeri → tampil penuh; geser kiri/kanan, Esc/klik untuk tutup.
  var links = Array.prototype.slice.call(document.querySelectorAll('a[data-lightbox]'));
  if (!links.length) return;
  var box = document.createElement('div');
  box.className = 'lv-lightbox';
  box.setAttribute('role', 'dialog');
  box.setAttribute('aria-modal', 'true');
  box.setAttribute('aria-label', 'Foto');
  box.hidden = true;
  box.innerHTML = '<button type="button" class="lv-lb-close" aria-label="Tutup">×</button>' +
    '<button type="button" class="lv-lb-prev" aria-label="Sebelumnya">‹</button>' +
    '<img alt=""><button type="button" class="lv-lb-next" aria-label="Berikutnya">›</button>';
  document.body.appendChild(box);
  var img = box.querySelector('img');
  var idx = 0;
  var startX = null;

  function show(i) {
    idx = (i + links.length) % links.length;
    img.src = links[idx].href;
    img.alt = (links[idx].querySelector('img') || {}).alt || '';
    box.hidden = false;
    document.body.style.overflow = 'hidden';
  }
  function close() {
    box.hidden = true;
    img.removeAttribute('src');
    document.body.style.overflow = '';
  }
  links.forEach(function (a, i) {
    a.addEventListener('click', function (e) { e.preventDefault(); show(i); });
  });
  box.querySelector('.lv-lb-close').addEventListener('click', close);
  box.querySelector('.lv-lb-prev').addEventListener('click', function (e) { e.stopPropagation(); show(idx - 1); });
  box.querySelector('.lv-lb-next').addEventListener('click', function (e) { e.stopPropagation(); show(idx + 1); });
  box.addEventListener('click', function (e) { if (e.target === box) close(); });
  document.addEventListener('keydown', function (e) {
    if (box.hidden) return;
    if (e.key === 'Escape') close();
    if (e.key === 'ArrowLeft') show(idx - 1);
    if (e.key === 'ArrowRight') show(idx + 1);
  });
  box.addEventListener('touchstart', function (e) { startX = e.touches[0].clientX; }, { passive: true });
  box.addEventListener('touchend', function (e) {
    if (startX === null) return;
    var dx = e.changedTouches[0].clientX - startX;
    if (Math.abs(dx) > 50) show(idx + (dx < 0 ? 1 : -1));
    startX = null;
  });
})();
