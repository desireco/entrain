// Keep bookmarks from the original single-page site working.
(function () {
  const routes = {
    '#studio': 'studio/', '#listening-guide': 'listening-guide/',
    '#handbook': 'handbook/', '#session-creator': 'studio/',
    '#session-source': 'studio/#session-source'
  };
  function followBookmark() {
    const route = routes[location.hash];
    if (!route) return;
    const destination = new URL(route, document.baseURI);
    destination.search = location.search;
    location.replace(destination.href);
  }
  followBookmark();
  window.addEventListener('hashchange', followBookmark);
})();
