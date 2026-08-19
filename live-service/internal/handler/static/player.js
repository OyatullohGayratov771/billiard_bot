// report — diagnostika ma'lumotini serverga yuboradi (mobil qurilmalarda
// muammo bo'lsa, sababini server log'idan ko'rish uchun). Hech qachon
// video ijrosiga xalaqit bermasligi uchun xatolar jimgina yutiladi.
function reportLive(tableSrc, event, data) {
  try {
    const payload = JSON.stringify({
      ts: new Date().toISOString(),
      src: tableSrc,
      event: event,
      ua: navigator.userAgent,
      data: data || {},
    });
    if (navigator.sendBeacon) {
      navigator.sendBeacon('/live/clientlog', payload);
    } else {
      fetch('/live/clientlog', { method: 'POST', body: payload, keepalive: true }).catch(() => {});
    }
  } catch (_) {}
}

// attachPlayer — HLS video oynatgichini ulaydi (hls.js yoki Safari-ning tabiiy
// HLS qo'llab-quvvatlashi orqali), yuklanish/xatolik holatlarini status
// qatlamida ko'rsatadi va uzilishlardan keyin avtomatik qayta urinadi.
function attachPlayer(video, statusEl, src) {
  function setStatus(text, isErr) {
    if (!statusEl) return;
    statusEl.textContent = text;
    statusEl.classList.toggle('err', !!isErr);
    statusEl.classList.remove('hide');
  }
  function hideStatus() {
    if (statusEl) statusEl.classList.add('hide');
  }

  reportLive(src, 'attach', {
    hlsSupported: !!(window.Hls && Hls.isSupported()),
    nativeHls: video.canPlayType('application/vnd.apple.mpegurl'),
  });

  // Ba'zi brauzerlar (ayniqsa iOS Safari) avtomatik play()ni sababsiz rad
  // etishi mumkin — bunda video tayyor turadi, lekin hech qachon boshlanmaydi
  // va "Ulanmoqda..." abadiy osilib qoladi. Shu holatda foydalanuvchiga
  // bosish orqali boshlash imkonini beramiz (bosish har doim ruxsat etiladi).
  function tryPlay() {
    const p = video.play();
    if (p && typeof p.catch === 'function') {
      p.catch((err) => {
        setStatus('▶️ Ko\'rish uchun bosing');
        reportLive(src, 'play_rejected', { name: err && err.name, message: err && err.message });
      });
    }
  }

  video.addEventListener('playing', () => { hideStatus(); reportLive(src, 'playing', { readyState: video.readyState }); });
  if (statusEl) {
    statusEl.style.cursor = 'pointer';
    statusEl.addEventListener('click', tryPlay);
  }
  video.addEventListener('click', tryPlay);

  let destroyed = false;
  let hls = null;

  function startHls() {
    if (destroyed) return;
    hls = new Hls({ liveSyncDurationCount: 3, maxBufferLength: 8 });
    hls.on(Hls.Events.MANIFEST_PARSED, tryPlay);
    hls.on(Hls.Events.ERROR, (_evt, data) => {
      reportLive(src, 'hls_error', { type: data.type, details: data.details, fatal: data.fatal });
      if (!data.fatal || destroyed) return;
      if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
        setStatus('⏳ Efir kutilmoqda...');
        setTimeout(() => { if (!destroyed && hls) hls.startLoad(); }, 2500);
      } else if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
        hls.recoverMediaError();
      } else {
        setStatus('⚠️ Ulanishda uzilish. Qayta urinilmoqda...', true);
        hls.destroy();
        setTimeout(startHls, 3000);
      }
    });
    hls.loadSource(src);
    hls.attachMedia(video);
  }

  function startNative() {
    video.src = src;
    video.load(); // dinamik yaratilgan <video> uchun iOS'da shart
    video.addEventListener('loadedmetadata', () => {
      reportLive(src, 'loadedmetadata', { duration: video.duration, readyState: video.readyState });
      tryPlay();
    }, { once: true });
    video.addEventListener('error', () => {
      reportLive(src, 'video_error', {
        code: video.error && video.error.code,
        message: video.error && video.error.message,
      });
      if (destroyed) return;
      setStatus('⏳ Efir kutilmoqda...');
      setTimeout(() => {
        if (destroyed) return;
        video.src = src;
        video.load();
      }, 2500);
    });

    // Xatolik ham, loadedmetadata ham hech qachon kelmasligi mumkin (jim
    // osilib qolish) — shu holatni ushlab, holatni serverga xabar qilamiz.
    let ticks = 0;
    const watchdog = setInterval(() => {
      if (destroyed) { clearInterval(watchdog); return; }
      ticks++;
      if (video.readyState > 0 || ticks > 6) { clearInterval(watchdog); return; }
      reportLive(src, 'watchdog', {
        readyState: video.readyState,
        networkState: video.networkState,
        currentSrc: video.currentSrc,
      });
    }, 4000);
  }

  if (window.Hls && Hls.isSupported()) {
    startHls();
    return { destroy() { destroyed = true; if (hls) hls.destroy(); } };
  } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
    startNative();
    return {
      destroy() {
        destroyed = true;
        video.removeAttribute('src');
        video.load();
      },
    };
  }
  reportLive(src, 'unsupported', {});
  setStatus("❌ Brauzeringiz video oqimini qo'llab-quvvatlamaydi.", true);
  return { destroy() {} };
}
