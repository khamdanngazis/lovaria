// Komponen Alpine milik Lovoria. Dimuat sebelum alpine.min.js (lihat layouts).
(function () {
  const MAX_BYTES = 10 * 1024 * 1024;
  const OK_TYPES = ['image/jpeg', 'image/png', 'image/webp'];

  function csrfToken() {
    const m = document.querySelector('meta[name="csrf-token"]');
    return m ? m.content : '';
  }

  // upload mengirim satu file via XHR (supaya ada progress). Resolve {status, body}.
  function upload(url, file, fields, accept, onProgress) {
    return new Promise((resolve) => {
      const fd = new FormData();
      fd.append('file', file);
      for (const [k, v] of Object.entries(fields)) fd.append(k, v);
      const xhr = new XMLHttpRequest();
      xhr.open('POST', url);
      xhr.setRequestHeader('X-CSRF-Token', csrfToken());
      xhr.setRequestHeader('Accept', accept);
      xhr.upload.onprogress = (e) => { if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100)); };
      xhr.onload = () => resolve({ status: xhr.status, body: xhr.responseText });
      xhr.onerror = () => resolve({ status: 0, body: 'Koneksi terputus. Coba lagi.' });
      xhr.send(fd);
    });
  }

  function precheck(file) {
    if (file.size > MAX_BYTES) return 'Ukuran file maksimal 10 MB';
    if (file.type && !OK_TYPES.includes(file.type)) return 'Hanya JPG, PNG, atau WebP';
    return '';
  }

  function errorText(res) {
    try { const j = JSON.parse(res.body); return j.error || j.message || 'Upload gagal'; } catch (_) { return res.body || 'Upload gagal'; }
  }

  document.addEventListener('alpine:init', () => {
    // Pengunggah banyak foto sekaligus: antrean, maks. 2 bersamaan, progress per file.
    // Salin teks (link undangan di beranda). Cadangan execCommand untuk
    // browser tanpa Clipboard API / konteks non-HTTPS.
    window.Alpine.data('copyText', (text) => ({
      copied: false,
      async copy() {
        let ok = false;
        try {
          await navigator.clipboard.writeText(text);
          ok = true;
        } catch (e) {
          const ta = document.createElement('textarea');
          ta.value = text;
          ta.style.cssText = 'position:fixed;opacity:0;font-size:16px';
          document.body.appendChild(ta);
          ta.select();
          ta.setSelectionRange(0, text.length);
          try { ok = document.execCommand('copy'); } catch (e2) { ok = false; }
          ta.remove();
        }
        this.copied = ok;
        if (ok) setTimeout(() => { this.copied = false; }, 2000);
      },
    }));

    // ---------- Bagikan undangan (T14) ----------
    const csrf = () => (document.querySelector('meta[name="csrf-token"]') || {}).content || '';
    async function copyToClipboard(text) {
      try {
        await navigator.clipboard.writeText(text);
        return true;
      } catch (e) {
        const ta = document.createElement('textarea');
        ta.value = text;
        ta.style.cssText = 'position:fixed;opacity:0;font-size:16px';
        document.body.appendChild(ta);
        ta.select();
        ta.setSelectionRange(0, text.length);
        let ok = false;
        try { ok = document.execCommand('copy'); } catch (e2) { ok = false; }
        ta.remove();
        return ok;
      }
    }
    // Baris tamu: salin link/pesan & tandai "sudah dibagikan" (POST, tanpa menunggu).
    window.Alpine.data('shareRow', (markURL, shared) => ({
      shared,
      copied: '',
      mark() {
        this.shared = true;
        fetch(markURL, { method: 'POST', headers: { 'X-CSRF-Token': csrf() }, keepalive: true }).catch(() => {});
      },
      async copy(text, what) {
        if (await copyToClipboard(text)) {
          this.copied = what;
          setTimeout(() => { this.copied = ''; }, 2000);
        }
        this.mark();
      },
    }));
    // Link umum: salin & Web Share API (menu bagikan bawaan ponsel).
    window.Alpine.data('shareGeneral', (link, text) => ({
      link,
      text,
      copied: '',
      canShare: !!navigator.share,
      async copy(value, what) {
        if (await copyToClipboard(value)) {
          this.copied = what;
          setTimeout(() => { this.copied = ''; }, 2000);
        }
      },
      nativeShare() {
        navigator.share({ text: this.text }).catch(() => {});
      },
    }));
    // Editor template: preview langsung dengan nilai contoh.
    window.Alpine.data('sharePreview', (cfg) => ({
      body: cfg.body,
      lang: cfg.lang,
      useDefault() { this.body = cfg.defaults[this.lang]; },
      preview() {
        const s = cfg.sample;
        return this.body
          .split('{guest_name}').join(s.guest_name)
          .split('{couple}').join(s.couple)
          .split('{date}').join(this.lang === 'en' ? s.date_en : s.date_id)
          .split('{link}').join(s.link);
      },
    }));

    window.Alpine.data('galleryUploader', (uploadURL, refreshURL) => ({
      category: 'wedding',
      files: [],
      running: 0,
      get busy() { return this.files.some((f) => f.status === 'waiting' || f.status === 'uploading'); },
      pick(event) {
        for (const file of event.target.files) {
          const err = precheck(file);
          this.files.push({ file, name: file.name, progress: 0, status: err ? 'error' : 'waiting', error: err });
        }
        event.target.value = '';
        this.pump();
      },
      pump() {
        while (this.running < 2) {
          const next = this.files.find((f) => f.status === 'waiting');
          if (!next) break;
          this.start(next);
        }
        if (!this.busy && this.running === 0 && this.files.some((f) => f.status === 'done' && !f.shown)) {
          this.files.forEach((f) => { if (f.status === 'done') f.shown = true; });
          window.htmx.ajax('GET', refreshURL, { target: '#gallery', swap: 'outerHTML' });
        }
      },
      async start(item) {
        item.status = 'uploading';
        this.running++;
        const res = await upload(uploadURL, item.file, { category: this.category }, 'application/json', (p) => { item.progress = p; });
        this.running--;
        if (res.status >= 200 && res.status < 300) {
          item.status = 'done';
          item.progress = 100;
        } else {
          item.status = 'error';
          item.error = errorText(res);
        }
        this.pump();
      },
      clearDone() { this.files = this.files.filter((f) => f.status === 'waiting' || f.status === 'uploading'); },
    }));

    // "Pilih dari kontak HP" (Contact Picker API — Chrome Android). Menambahkan
    // "Nama Nomor" per kontak ke textarea x-ref="text".
    // Form tamu: pilih SATU kontak HP → isi No. HP (dan Nama bila masih kosong).
    // Contact Picker API hanya ada di Chrome/Edge/Samsung Internet Android;
    // tombol disembunyikan di browser lain. Normalisasi nomor tetap di server.
    window.Alpine.data('contactFill', () => ({
      supported: 'contacts' in navigator && 'select' in navigator.contacts,
      async pick() {
        try {
          const [c] = await navigator.contacts.select(['name', 'tel'], { multiple: false });
          if (!c) return;
          const tel = (c.tel || [])[0];
          const name = (c.name || [])[0];
          const phoneInput = this.$root.querySelector('[name="phone"]');
          const nameInput = this.$root.querySelector('[name="name"]');
          if (tel && phoneInput) phoneInput.value = tel;
          if (name && nameInput && !nameInput.value.trim()) nameInput.value = name;
          (nameInput && !nameInput.value.trim() ? nameInput : phoneInput)?.focus();
        } catch (_) { /* dibatalkan user */ }
      },
    }));

    window.Alpine.data('contactPicker', () => ({
      supported: 'contacts' in navigator && 'select' in navigator.contacts,
      async pick() {
        try {
          const picked = await navigator.contacts.select(['name', 'tel'], { multiple: true });
          const lines = picked
            .map((c) => [(c.name || [])[0], (c.tel || [])[0]].filter(Boolean).join(' '))
            .filter(Boolean);
          if (!lines.length) return;
          const ta = this.$refs.text;
          ta.value = (ta.value.trim() ? ta.value.trimEnd() + '\n' : '') + lines.join('\n');
        } catch (_) { /* dibatalkan user */ }
      },
    }));

    // Preview tema: perbarui iframe#theme-preview dari isi form (dengan jeda).
    window.Alpine.data('themePreview', (previewURL) => ({
      timer: null,
      refresh() {
        clearTimeout(this.timer);
        this.timer = setTimeout(() => {
          const params = new URLSearchParams(new FormData(this.$refs.form));
          params.delete('_csrf');
          params.delete('_method');
          document.getElementById('theme-preview').src = previewURL + '?' + params.toString();
        }, 400);
      },
    }));

    // Warna opsional: checkbox "pakai warna sendiri" + color picker.
    window.Alpine.data('colorField', (initial) => ({
      custom: !!initial,
      color: initial || '#b76e79',
    }));

    // Field satu foto: upload lalu isi input tersembunyi dengan URL hasilnya.
    window.Alpine.data('imageUpload', (uploadURL, category, initial) => ({
      value: initial || '',
      progress: 0,
      uploading: false,
      error: '',
      async pick(event) {
        const file = event.target.files[0];
        event.target.value = '';
        if (!file) return;
        this.error = precheck(file);
        if (this.error) return;
        this.uploading = true;
        this.progress = 0;
        const res = await upload(uploadURL, file, { category }, 'application/json', (p) => { this.progress = p; });
        this.uploading = false;
        if (res.status >= 200 && res.status < 300) {
          this.value = JSON.parse(res.body).url;
          this.$nextTick(() => this.$dispatch('image-change'));
        } else {
          this.error = errorText(res);
        }
      },
    }));
  });
})();

// Error dari server untuk request htmx (4xx/5xx tidak di-swap, kecuali 422/429):
// tampilkan pesan singkat supaya aksi yang gagal tidak diam saja (mis. mode
// lihat saja admin). Server mengirim pesan sebagai teks biasa untuk htmx.
(() => {
  let box, timer;
  const show = (msg) => {
    if (!box) {
      box = document.createElement('div');
      box.setAttribute('role', 'alert');
      box.className = 'fixed inset-x-4 bottom-20 z-[60] mx-auto max-w-md rounded-xl bg-red-600 px-4 py-3 text-center text-sm text-white shadow-lg lg:bottom-6';
      document.body.appendChild(box);
    }
    box.textContent = msg;
    box.hidden = false;
    clearTimeout(timer);
    timer = setTimeout(() => { box.hidden = true; }, 5000);
  };
  document.body.addEventListener('htmx:responseError', (e) => {
    const text = (e.detail.xhr && e.detail.xhr.responseText) || '';
    // Hanya teks pendek; halaman HTML penuh (mis. dari proxy) diganti pesan umum.
    show(text && text.length < 300 && !/</.test(text) ? text : 'Terjadi kesalahan. Coba lagi.');
  });
  document.body.addEventListener('htmx:sendError', () => show('Tidak bisa terhubung ke server. Periksa koneksi internet.'));
})();

// Pengganti handler inline (CSP melarang onsubmit=/onfocus= di HTML):
// <form data-confirm="Yakin?"> dan <input data-select-on-focus>.
document.addEventListener('submit', (e) => {
  const msg = e.target.dataset && e.target.dataset.confirm;
  if (msg && !window.confirm(msg)) {
    e.preventDefault();
    e.stopImmediatePropagation();
  }
}, true);
document.addEventListener('focusin', (e) => {
  if (e.target.matches && e.target.matches('[data-select-on-focus]')) e.target.select();
});
