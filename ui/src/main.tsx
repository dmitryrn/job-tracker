import { createRoot } from "react-dom/client";
import { Toaster } from "sonner";
import App from "./App";

createRoot(document.getElementById("root")!).render(
  <>
    <App />
    <Toaster position="top-center" richColors closeButton />
  </>,
);
