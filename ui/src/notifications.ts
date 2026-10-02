import { toast } from "sonner";

export function notifyError(reason: unknown, fallback: string) {
  const message = reason instanceof Error ? reason.message : typeof reason === "string" ? reason : fallback;
  toast.error(message);
  return message;
}

export function notifySuccess(message: string) {
  toast.success(message);
}
