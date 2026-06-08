import path from "node:path";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import VueI18nPlugin from "@intlify/unplugin-vue-i18n/vite";
import legacy from "@vitejs/plugin-legacy";
import { compression } from "vite-plugin-compression2";

const plugins = [
  vue(),
  VueI18nPlugin({
    include: [path.resolve(__dirname, "./src/i18n/**/*.json")],
  }),
  legacy({
    // defaults already drop IE support
    targets: ["defaults"],
  }),
  // Explicit `algorithm: "gzip"` — the v2 plugin default produces `.br`
  // only for modern chunks, but http/static.go serves `.gz` exclusively.
  // Without this, every dynamic-import chunk that lacks a `.gz` sibling
  // 404s on prod and the SPA hangs trying to load mermaid / lodash / etc.
  compression({
    algorithm: "gzip",
    include: /\.js$/,
    deleteOriginalAssets: false,
  }),
];

const resolve = {
  alias: {
    // vue: "@vue/compat",
    "@/": `${path.resolve(__dirname, "src")}/`,
  },
};

// https://vitejs.dev/config/
export default defineConfig(({ command }) => {
  if (command === "serve") {
    // Dev proxy target — override via VITE_BACKEND env var so the dev
    // backend can run on a non-default port (e.g. when :8080 is held by
    // a locally-installed prod filebrowser binary).
    const backendHTTP = process.env.VITE_BACKEND || "http://127.0.0.1:8080";
    const backendWS = backendHTTP.replace(/^http/, "ws");
    return {
      plugins,
      resolve,
      server: {
        proxy: {
          "/api/command": {
            target: `${backendWS}`,
            ws: true,
          },
          "/api": backendHTTP,
        },
      },
    };
  } else {
    // command === 'build'
    return {
      plugins,
      resolve,
      base: "",
      build: {
        rollupOptions: {
          input: {
            index: path.resolve(__dirname, "./public/index.html"),
          },
          output: {
            manualChunks: (id) => {
              // bundle dayjs files in a single chunk
              // this avoids having small files for each locale
              if (id.includes("dayjs/")) {
                return "dayjs";
                // bundle i18n in a separate chunk
              } else if (id.includes("i18n/")) {
                return "i18n";
              }
            },
          },
        },
      },
      experimental: {
        renderBuiltUrl(filename, { hostType }) {
          if (hostType === "js") {
            return { runtime: `window.__prependStaticUrl("${filename}")` };
          } else if (hostType === "html") {
            return `[{[ .StaticURL ]}]/${filename}`;
          } else {
            return { relative: true };
          }
        },
      },
    };
  }
});
