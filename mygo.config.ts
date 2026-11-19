// The MyGo app configuration. The CLI evaluates this file with Bun, so a
// plain default export is all it needs. Keep mygo.json out of the project:
// the two would configure the app twice.
import { defineConfig } from "mygo-cli";

export default defineConfig({
  name: "存号网络",
  slug: "cloud-phone",
  identifier: "com.example.cloudphone",
  version: "0.1.0",
  copyright: "© 2026 存号网络",

  // `go tool mygo dev` runs devCommand and loads devUrl; `go tool mygo build`
  // runs buildCommand and embeds frontendDist into the executable.
  devUrl: "http://localhost:5173",
  devCommand: "bun run dev:web",
  buildCommand: "bun run build:web",
  frontendDist: "dist",
  // `go tool mygo generate` writes the typed client of the Go services here.
  bindings: "src/mygo.ts",
  out: "build",

  linux: {
    maintainer: "存号网络",
    comment: "云手机运营平台",
    categories: ["Office", "Network"],
  },
});
