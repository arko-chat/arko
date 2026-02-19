import "htmx.org";
import "htmx-ext-ws";
import "htmx-ext-loading-states";
import "alpinejs";

document.addEventListener("alpine:init", () => {
  console.log("Alpine.js initialized");
});

window.addEventListener("load", () => {
  console.log(
    "Window loaded. Alpine available:",
    typeof Alpine !== "undefined",
  );
});

document.body.addEventListener("htmx:afterSettle", function(evt) {
  var t = document.querySelector("title");
  if (t) document.title = t.textContent;
});

document.addEventListener("htmx:wsBeforeMessage", function(e) {
  const socketEl = e.target;
  if (socketEl && socketEl.closest("[hx-swap-oob]")) {
    return;
  }
});

document.addEventListener("htmx:beforeSwap", function(e) {
  const currentWs = document.querySelector("[ws-connect]");
  if (!currentWs) return;

  const target = e.detail.target;
  if (target && target.contains(currentWs)) return;

  const ext = currentWs.getAttribute("ws-connect");
  if (ext) {
    htmx.trigger(currentWs, "htmx:wsClose");
  }
});
