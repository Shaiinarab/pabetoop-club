const CACHE_NAME = "pabetoop-static-v2";
const STATIC_ASSETS = [
  "/assets/manifest.webmanifest",
  "/assets/js/htmx-4.0.0-beta6.min.js",
  "/assets/js/pwa-register.js",
  "/assets/icons/icon-192.svg",
  "/assets/icons/icon-512.svg"
];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(STATIC_ASSETS))
  );
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key)))
      )
      .then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (event) => {
  const request = event.request;
  if (request.method !== "GET") {
    return;
  }

  const url = new URL(request.url);
  if (url.origin !== self.location.origin) {
    return;
  }

  if (isSensitiveRoute(url.pathname)) {
    event.respondWith(fetch(request));
    return;
  }

  if (url.pathname.startsWith("/assets/")) {
    event.respondWith(cacheFirst(request));
    return;
  }

  event.respondWith(networkFirst(request));
});

function isSensitiveRoute(pathname) {
  return (
    pathname === "/portal" ||
    pathname.startsWith("/portal/") ||
    pathname === "/login" ||
    pathname === "/logout" ||
    pathname === "/manager" ||
    pathname.startsWith("/manager/") ||
    pathname.startsWith("/pay/") ||
    pathname.startsWith("/payments/")
  );
}

async function cacheFirst(request) {
  const cached = await caches.match(request);
  if (cached) {
    return cached;
  }
  const response = await fetch(request);
  if (response.ok) {
    const cache = await caches.open(CACHE_NAME);
    cache.put(request, response.clone());
  }
  return response;
}

async function networkFirst(request) {
  try {
    const response = await fetch(request);
    if (response.ok && isPublicHTML(request)) {
      const cache = await caches.open(CACHE_NAME);
      cache.put(request, response.clone());
    }
    return response;
  } catch (error) {
    const cached = await caches.match(request);
    if (cached) {
      return cached;
    }
    throw error;
  }
}

function isPublicHTML(request) {
  return request.mode === "navigate" && request.url === self.location.origin + "/";
}
