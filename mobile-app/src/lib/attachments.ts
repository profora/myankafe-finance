export const MAX_ATTACHMENT_BYTES = 20 * 1024 * 1024;

export const ALLOWED_MIME = new Set([
  "image/jpeg",
  "image/png",
  "image/webp",
  "image/gif",
  "application/pdf",
  "text/plain",
  "text/csv",
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
]);

export function attachmentError(file: { mime?: string | null; size?: number | null }): string | null {
  if (file.mime && !ALLOWED_MIME.has(file.mime)) return "That file type is not accepted.";
  if (file.size != null && file.size > MAX_ATTACHMENT_BYTES) return "Files must be 20 MB or smaller.";
  return null;
}
