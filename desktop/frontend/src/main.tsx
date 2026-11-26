import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "@fontsource-variable/manrope";
import "@fontsource-variable/unbounded";
import "./index.css";
import App from "./App";
import { ErrorBoundary } from "./components/ErrorBoundary";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <div className="h-full p-0">
      <ErrorBoundary>
        <App />
      </ErrorBoundary>
    </div>
  </StrictMode>,
);
