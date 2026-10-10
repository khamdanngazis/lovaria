// Pemindai QR check-in tamu (T31) — halaman penerima tamu, tanpa Alpine.
// Membaca QR dengan BarcodeDetector bila tersedia (Chrome Android); selain itu
// memuat jsQR (Safari iOS). Hasil dikirim ke server lewat htmx dan ditampilkan
// di #checkin-result.
(function () {
  'use strict';
  var root = document.getElementById('checkin');
  var video = document.getElementById('checkin-video');
  var startBtn = document.getElementById('checkin-start');
  var result = document.getElementById('checkin-result');
  if (!root || !video || !startBtn || !result) return;

  var idle = document.getElementById('checkin-idle');
  var statusEl = document.getElementById('checkin-status');
  var detector = null;
  var canvas = null;
  var paused = false; // menunggu keputusan penerima tamu / jawaban server
  var lastText = '';
  var lastAt = 0;

  function status(text) {
    if (statusEl) statusEl.textContent = text;
  }

  function loadScript(src) {
    return new Promise(function (resolve, reject) {
      var s = document.createElement('script');
      s.src = src;
      s.onload = resolve;
      s.onerror = reject;
      document.head.appendChild(s);
    });
  }

  function readFrame() {
    if (detector) {
      return detector.detect(video).then(function (codes) {
        return codes.length ? codes[0].rawValue : '';
      });
    }
    if (!window.jsQR || !video.videoWidth) return Promise.resolve('');
    // Perkecil bingkai: cukup untuk QR di layar ponsel, ringan untuk perangkat lama.
    var scale = Math.min(1, 640 / Math.max(video.videoWidth, video.videoHeight));
    var w = Math.round(video.videoWidth * scale);
    var h = Math.round(video.videoHeight * scale);
    canvas = canvas || document.createElement('canvas');
    canvas.width = w;
    canvas.height = h;
    var ctx = canvas.getContext('2d', { willReadFrequently: true });
    ctx.drawImage(video, 0, 0, w, h);
    var found = window.jsQR(ctx.getImageData(0, 0, w, h).data, w, h, { inversionAttempts: 'dontInvert' });
    return Promise.resolve(found ? found.data : '');
  }

  function submit(text) {
    paused = true;
    status('Memeriksa…');
    if (navigator.vibrate) navigator.vibrate(60);
    window.htmx
      .ajax('POST', root.dataset.scanUrl, { target: '#checkin-result', swap: 'innerHTML', values: { code: text, via: 'scan' } })
      .catch(function () {
        failed();
      });
  }

  function failed() {
    result.innerHTML = '<p class="ui-alert ui-alert-danger" role="alert">Koneksi bermasalah. Periksa sinyal, lalu coba lagi.</p>';
    paused = false;
    status('Siap memindai');
  }

  function tick() {
    if (paused || video.readyState < 2) return;
    readFrame()
      .then(function (text) {
        if (!text || paused) return;
        var now = Date.now();
        if (text === lastText && now - lastAt < 4000) return; // QR yang sama masih di depan kamera
        lastText = text;
        lastAt = now;
        submit(text);
      })
      .catch(function () {});
  }

  function start() {
    if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
      status('Kamera tidak tersedia di browser ini. Pakai "Cari manual" di bawah.');
      return;
    }
    status('Meminta izin kamera…');
    navigator.mediaDevices
      .getUserMedia({ video: { facingMode: { ideal: 'environment' } }, audio: false })
      .then(function (stream) {
        video.srcObject = stream;
        return video.play();
      })
      .then(function () {
        if ('BarcodeDetector' in window) {
          return window.BarcodeDetector.getSupportedFormats().then(function (formats) {
            if (formats.indexOf('qr_code') >= 0) detector = new window.BarcodeDetector({ formats: ['qr_code'] });
          });
        }
      })
      .catch(function (err) {
        if (!video.srcObject) throw err; // izin kamera ditolak
      })
      .then(function () {
        if (!detector && !window.jsQR) return loadScript(root.dataset.jsqr);
      })
      .then(function () {
        if (idle) idle.hidden = true;
        status('Siap memindai');
        window.setInterval(tick, 250);
      })
      .catch(function () {
        status('Kamera tidak bisa dinyalakan. Izinkan akses kamera, atau pakai "Cari manual" di bawah.');
      });
  }

  startBtn.addEventListener('click', start);

  // Setelah hasil tampil: pemindai berhenti selama ada kartu hasil
  // (data-scan-pause) dan lanjut saat "Pindai berikutnya" ditekan.
  document.body.addEventListener('htmx:afterSwap', function (e) {
    if (e.target !== result) return;
    paused = !!result.querySelector('[data-scan-pause]');
    status(paused ? '' : 'Siap memindai');
    result.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  });
  // Form bertanda data-reset-on-success (tamu tambahan) dikosongkan setelah terkirim.
  document.body.addEventListener('htmx:afterRequest', function (e) {
    if (e.detail.successful && e.target.matches && e.target.matches('form[data-reset-on-success]')) e.target.reset();
  });
  document.body.addEventListener('htmx:sendError', failed);
  document.body.addEventListener('htmx:responseError', failed);

  // "Pindai berikutnya" / "Batal": bersihkan hasil dan lanjut memindai.
  result.addEventListener('click', function (e) {
    if (!e.target.closest('[data-scan-resume]')) return;
    result.innerHTML = '';
    paused = false;
    lastText = '';
    status('Siap memindai');
  });
})();
