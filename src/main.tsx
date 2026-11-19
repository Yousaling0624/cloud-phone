import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import "./style.css";

const container = document.getElementById("root");
if (!container) throw new Error("missing #root");

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);

// The boot screen sits above the app until the first React frame paints.
const boot = document.getElementById("boot");
if (boot) {
  requestAnimationFrame(() => boot.remove());
}
