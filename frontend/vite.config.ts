import path from "node:path";
import react from "@vitejs/plugin-react";
import { defineConfig, type PluginOption } from "vite";

function assetVersionPlugin(): PluginOption {
  const version = process.env.GARGOYLE_FRONTEND_BUILD ?? Date.now().toString(36);
  return {
    name: "gargoyle-asset-version",
    apply: "build",
    transformIndexHtml: {
      order: "post",
      handler(html) {
        return html.replace(/((?:src|href)=\"\/assets\/[^\"]+\.(?:js|css))(\")/g, `$1?v=${version}$2`);
      },
    },
  };
}

export default defineConfig({
  plugins: [react(), assetVersionPlugin()],
  server: {
    proxy: {
      "/api": {
        target: "http://gargoyle.test",
        changeOrigin: true,
        configure(proxy) {
          proxy.on("proxyReq", (proxyReq) => {
            proxyReq.removeHeader("cookie");
          });
        },
      },
      "/oauth/token": {
        target: "http://gargoyle.test",
        changeOrigin: true,
        configure(proxy) {
          proxy.on("proxyReq", (proxyReq) => {
            proxyReq.removeHeader("cookie");
          });
        },
      },
      "/oauth/revoke": {
        target: "http://gargoyle.test",
        changeOrigin: true,
        configure(proxy) {
          proxy.on("proxyReq", (proxyReq) => {
            proxyReq.removeHeader("cookie");
          });
        },
      },
      "/media": {
        target: "http://gargoyle.test",
        changeOrigin: true,
        configure(proxy) {
          proxy.on("proxyReq", (proxyReq) => {
            proxyReq.removeHeader("cookie");
          });
        },
      },
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
