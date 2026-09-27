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
        } else {
          this.error = errorText(res);
        }
      },
    }));
  });
})();
