import { QueryClient } from "@tanstack/react-query";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 15_000,
      gcTime: 5 * 60_000,
    },
  },
});

/** Logout and expired sessions must not leave another user's books in memory. */
export function clearFinanceCache() {
  queryClient.clear();
}
