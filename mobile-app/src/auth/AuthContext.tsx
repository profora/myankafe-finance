import { createContext, ReactNode, useContext, useEffect, useMemo, useState } from "react";
import { api, setUnauthorizedHandler } from "../api/runtime";
import { clearFinanceCache } from "../lib/queryClient";
import { clearSession, getSessionToken, saveSession } from "./session";
import { friendlyError } from "../api/errors";

export type SessionUser = {
  public_id: string;
  username: string;
  display_name: string;
};

type AuthValue = {
  ready: boolean;
  user: SessionUser | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  replaceToken: (token: string, expiresAt?: string) => Promise<void>;
};

const AuthContext = createContext<AuthValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false);
  const [user, setUser] = useState<SessionUser | null>(null);

  useEffect(() => {
    setUnauthorizedHandler(() => {
      clearFinanceCache();
      setUser(null);
    });
    (async () => {
      const token = await getSessionToken();
      if (!token) {
        setReady(true);
        return;
      }
      try {
        const me = await api().request<{ user: SessionUser }>("/auth/me");
        setUser(me.user);
      } catch {
        await clearSession();
        setUser(null);
      } finally {
        setReady(true);
      }
    })();
  }, []);

  const value = useMemo<AuthValue>(
    () => ({
      ready,
      user,
      async login(username, password) {
        try {
          const result = await api().login(username.trim(), password);
          if (!result.session_token) throw new Error("The server did not return a session token.");
          await saveSession(result.session_token, result.session_expires_at);
          setUser(result.user);
        } catch (err) {
          throw new Error(friendlyError(err));
        }
      },
      async logout() {
        try {
          await api().request("/auth/logout", { method: "POST", body: "{}" });
        } catch {
          /* local session still clears */
        }
        clearFinanceCache();
        await clearSession();
        setUser(null);
      },
      async replaceToken(token, expiresAt) {
        await saveSession(token, expiresAt);
      },
    }),
    [ready, user],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("AuthProvider missing");
  return value;
}
