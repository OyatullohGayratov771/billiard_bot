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

  // Ba'zi brauzerlar (ayniqsa iOS Safari) avtomatik play()ni sababsiz rad
  // etishi mumkin — bunda video tayyor turadi, lekin hech qachon boshlanmaydi
  // va "Ulanmoqda..." abadiy osilib qoladi. Shu holatda foydalanuvchiga
  // bosish orqali boshlash imkonini beramiz (bosish har doim ruxsat etiladi).
  function tryPlay() {
    const p = video.play();
    if (p && typeof p.catch === 'function') {
      p.catch(() => {
        setStatus('▶️ Ko\'rish uchun bosing');
      });
    }
  }

  video.addEventListener('playing', hideStatus);
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
    video.addEventListener('loadedmetadata', tryPlay, { once: true });
    video.addEventListener('error', () => {
      if (destroyed) return;
      setStatus('⏳ Efir kutilmoqda...');
      setTimeout(() => {
        if (destroyed) return;
        video.src = src;
        video.load();
      }, 2500);
    });
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
  setStatus("❌ Brauzeringiz video oqimini qo'llab-quvvatlamaydi.", true);
  return { destroy() {} };
}
