import "htmx-ext-ws";
import "htmx-ext-loading-states";
import htmx from "htmx.org";
import Alpine from "alpinejs";
import type { Alpine as AlpineType } from "alpinejs";
import 'github-markdown-css/github-markdown.css';

declare global {
  interface Window {
    Alpine: AlpineType;
  }
}

window.Alpine = Alpine

Alpine.start()

var s = document.documentElement.style;
var keys = [
  "nav-sidebar-width",
  "activity-sidebar-width",
  "members-sidebar-width",
];
keys.forEach(function(k) {
  var v = localStorage.getItem(k);
  if (v) s.setProperty("--" + k, v + "px");
});

document.addEventListener("alpine:init", () => {
  console.log("Alpine.js initialized");
});

window.addEventListener("load", () => {
  console.log(
    "Window loaded. Alpine available:",
    typeof window.Alpine !== "undefined",
  );
});

document.body.addEventListener("htmx:afterSettle", (evt: Event) => {
  const t = document.querySelector("title");
  if (t) document.title = t.textContent ?? "";
});

document.addEventListener("htmx:wsBeforeMessage", (e: Event) => {
  const socketEl = e.target as HTMLElement | null;
  if (socketEl && socketEl.closest("[hx-swap-oob]")) {
    return;
  }
});

document.addEventListener("htmx:beforeSwap", (e: Event) => {
  const detail = (e as CustomEvent).detail as { target: HTMLElement | null };
  const currentWs = document.querySelector<HTMLElement>("[ws-connect]");
  if (!currentWs) return;

  const target = detail.target;
  if (target && target.contains(currentWs)) return;

  const ext = currentWs.getAttribute("ws-connect");
  if (ext) {
    htmx.trigger(currentWs, "htmx:wsClose");
  }
});


