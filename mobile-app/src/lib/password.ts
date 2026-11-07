export function passwordChangeError(current: string, next: string): string | null {
  if (!current) return "Enter your current password.";
  const length = Array.from(next).length;
  if (length < 12) return "New password must be at least 12 characters.";
  if (length > 128) return "New password must be at most 128 characters.";
  return null;
}
