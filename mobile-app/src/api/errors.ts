export class ApiError extends Error {
  status: number;
  code?: string;
  constructor(message: string, status: number, code?: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export function friendlyError(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 401) return "Sign in again to continue.";
    if (err.status === 403) return "You do not have permission for that action.";
    if (err.status === 429) return "Too many attempts. Wait a moment and try again.";
    if (err.status === 0) return "Network unavailable. Check your connection and try again.";
    return err.message || "Request failed.";
  }
  if (err instanceof Error && /network|failed to fetch/i.test(err.message)) {
    return "Network unavailable. Check your connection and try again.";
  }
  return "Something went wrong. Try again.";
}
