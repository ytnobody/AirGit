const CACHE_NAME = 'airgit-v2';
const ASSETS_TO_CACHE = [
  '/',
  '/index.html',
  '/manifest.json',
  '/icon.png'
];

self.addEventListener('install', (event) => {
  console.log('Service Worker: Installing...');
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      console.log('Service Worker: Caching app shell');
      return cache.addAll(ASSETS_TO_CACHE).catch((error) => {
        console.error('Service Worker: Cache addAll error:', error);
        return Promise.resolve();
      });
    })
  );
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  console.log('Service Worker: Activating...');
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames.map((cacheName) => {
          if (cacheName !== CACHE_NAME) {
            console.log('Service Worker: Deleting old cache:', cacheName);
            return caches.delete(cacheName);
          }
        })
      );
    })
  );
  self.clients.claim();
});

self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') {
    return;
  }

  const url = new URL(event.request.url);
  
  // API calls should always go to network
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(
      fetch(event.request).catch(() => {
        return caches.match('/');
      })
    );
    return;
  }

  // For static assets, use cache-first strategy with network fallback
  event.respondWith(
    caches.match(event.request).then((response) => {
      if (response) {
        // If cached, fetch fresh version in background
        fetch(event.request).then((freshResponse) => {
          if (freshResponse && freshResponse.status === 200) {
            caches.open(CACHE_NAME).then((cache) => {
              cache.put(event.request, freshResponse);
            });
          }
        }).catch(() => {
          // Ignore network errors for background refresh
        });
        return response;
      }
      
      // Not cached, fetch from network
      return fetch(event.request).then((response) => {
        // Cache successful responses
        if (response && response.status === 200) {
          const responseToCache = response.clone();
          caches.open(CACHE_NAME).then((cache) => {
            cache.put(event.request, responseToCache);
          });
        }
        return response;
      }).catch(() => {
        // Network error, fallback to cache or home
        return caches.match('/');
      });
    })
  );
});

self.addEventListener('message', (event) => {
  console.log('Service Worker: Received message:', event.data);
});

console.log('Service Worker: Script loaded');