import "htmx-ext-ws";
import "htmx-ext-loading-states";
import "htmx-ext-response-targets";
import htmx from "htmx.org";
import Alpine from "alpinejs";
import type { Alpine as AlpineType } from "alpinejs";

declare global {
  interface Window {
    Alpine: AlpineType;
  }
}

window.Alpine = Alpine;

interface FileEntry {
  file: File;
  name: string;
  sizeLabel: string;
  isImage: boolean;
  previewURL: string | null;
}

interface MessageInputData {
  pendingFiles: FileEntry[];
  onFilesSelected(fileList: FileList | File[]): void;
  removeFile(index: number): void;
  onSubmit(e: SubmitEvent): void;
}

function messageInput(): MessageInputData {
  return {
    pendingFiles: [],

    onFilesSelected(fileList: FileList | File[]) {
      for (const file of Array.from(fileList)) {
        this.pendingFiles.push(buildFileEntry(file));
      }
    },

    removeFile(index: number) {
      const entry = this.pendingFiles[index];
      if (entry?.previewURL) URL.revokeObjectURL(entry.previewURL);
      this.pendingFiles.splice(index, 1);
    },

    onSubmit(e: SubmitEvent) {
      console.log("i am submitting")
      if (this.pendingFiles.length === 0) return;
      const form = e.target as HTMLFormElement;
      const fd = new FormData(form);
      for (const entry of this.pendingFiles) {
        fd.append("attachments", entry.file, entry.name);
      }
      fetch(form.action || window.location.pathname, {
        method: "POST",
        body: fd,
      }).then(() => {
        for (const entry of this.pendingFiles) {
          if (entry.previewURL) URL.revokeObjectURL(entry.previewURL);
        }
        this.pendingFiles = [];
        form.reset();
      });
    },
  };
}

interface ChatDropData {
  dragOver: boolean;
  _counter: number;
  onDragOver(e: DragEvent): void;
  onDragLeave(e: DragEvent): void;
  onDrop(e: DragEvent): void;
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function buildFileEntry(file: File): FileEntry {
  const isImage = file.type.startsWith("image/");
  return {
    file,
    name: file.name,
    sizeLabel: formatBytes(file.size),
    isImage,
    previewURL: isImage ? URL.createObjectURL(file) : null,
  };
}

function chatDrop(): ChatDropData {
  return {
    dragOver: false,
    _counter: 0,

    onDragOver(e: DragEvent) {
      if (!e.dataTransfer?.types.includes("Files")) return;
      this._counter++;
      this.dragOver = true;
    },

    onDragLeave(_e: DragEvent) {
      this._counter--;
      if (this._counter <= 0) {
        this._counter = 0;
        this.dragOver = false;
      }
    },

    onDrop(e: DragEvent) {
      this._counter = 0;
      this.dragOver = false;

      const files = e.dataTransfer?.files;
      if (!files?.length) return;

      const formEl = document.querySelector<HTMLElement>("form[x-data]");
      if (!formEl) return;

      const inputComp = Alpine.$data(formEl) as
        | MessageInputData
        | undefined;
      inputComp?.onFilesSelected(files);
    },
  };
}

document.addEventListener("alpine:init", () => {
  console.log("Alpine.js initialized");

  Alpine.data("messageInput", messageInput);
  Alpine.data("chatDrop", chatDrop);
});

Alpine.start();

const s = document.documentElement.style;
const keys = [
  "nav-sidebar-width",
  "activity-sidebar-width",
  "members-sidebar-width",
];
keys.forEach((k) => {
  const v = localStorage.getItem(k);
  if (v) s.setProperty(`--${k}`, `${v}px`);
});

window.addEventListener("load", () => {
  console.log(
    "Window loaded. Alpine available:",
    typeof window.Alpine !== "undefined"
  );
});

document.body.addEventListener("htmx:afterSettle", (_evt: Event) => {
  const t = document.querySelector("title");
  if (t) document.title = t.textContent ?? "";
});

document.addEventListener("htmx:wsBeforeMessage", (e: Event) => {
  const detail = (e as CustomEvent<{ message: string }>).detail;
  try {
    const msg = JSON.parse(detail.message) as { redirect?: string };
    if (msg.redirect) {
      window.location.href = msg.redirect;
    }
  } catch (_) { }

  const socketEl = e.target as HTMLElement | null;
  if (socketEl && socketEl.closest("[hx-swap-oob]")) {
    return;
  }
});

document.addEventListener("htmx:beforeSwap", (e: Event) => {
  const detail = (e as CustomEvent).detail as {
    target: HTMLElement | null;
  };
  const currentWs = document.querySelector<HTMLElement>("[ws-connect]");
  if (!currentWs) return;

  const target = detail.target;
  if (target && target.contains(currentWs)) return;

  const ext = currentWs.getAttribute("ws-connect");
  if (ext) {
    htmx.trigger(currentWs, "htmx:wsClose");
  }
});
