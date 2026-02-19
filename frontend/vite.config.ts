import { defineConfig } from "vite";

export default defineConfig({
  build: {
    manifest: true,
    outDir: "../components/assets/dist",
    emptyOutDir: true,
    rollupOptions: {
      input: {
        main: "src/assets/js/index.ts",
        styles: "src/assets/css/app.css",
      },
    },
  },
});
