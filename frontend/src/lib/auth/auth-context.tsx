"use client";

import { authApi, type UserProfile } from "./auth-api";
import { esRutaPublica } from "./rutas-publicas";
import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";

const TOKEN_KEY = "auth_token";

interface AuthState {
  user: UserProfile | null;
  token: string | null;
  loading: boolean;
}

interface AuthContextValue extends AuthState {
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  updateUser: (patch: Partial<UserProfile>) => void;
  isAuthenticated: boolean;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>({ user: null, token: null, loading: true });

  // Restore session from localStorage on mount. The request is silent: a stale
  // token is cleared here without firing "auth:expired", so opening a public
  // page (an e-mailed verify-email / password-new link) with an old token in
  // localStorage never redirects. Private pages are guarded by the dashboard
  // layout, which sends a session-less visitor to sign-in.
  useEffect(() => {
    const stored = localStorage.getItem(TOKEN_KEY);
    if (!stored) {
      setState((s) => ({ ...s, loading: false }));
      return;
    }
    authApi
      .getMe(stored, { silencioso: true })
      .then((user) => {
        setState({ user, token: stored, loading: false });
      })
      .catch((err: { status?: number }) => {
        // Only a 401 means the session is gone. A network error, 5xx or 429
        // keeps the token: the session stays open with `user` pending, and the
        // polling below retries /auth/me until it answers.
        if (err?.status === 401) {
          localStorage.removeItem(TOKEN_KEY);
          setState({ user: null, token: null, loading: false });
        } else {
          setState({ user: null, token: stored, loading: false });
        }
      });
  }, []);

  // Handle token expiry from any API call: the session is always cleared, but
  // only a private page redirects to sign-in. On a public page (`/`, `/auth/*`)
  // the visitor stays put, so a link whose token was already removed from the
  // URL is not lost.
  useEffect(() => {
    const handler = () => {
      localStorage.removeItem(TOKEN_KEY);
      setState({ user: null, token: null, loading: false });
      if (!esRutaPublica(window.location.pathname)) window.location.replace("/auth/sign-in");
    };
    window.addEventListener("auth:expired", handler);
    return () => window.removeEventListener("auth:expired", handler);
  }, []);

  // Poll /auth/me every 60s to detect session revocation from another device.
  // Tied to the token in state, so it also runs after a login in this tab.
  // While `user` is still pending (the restore failed for a non-401 reason)
  // it retries every 5s and fills it in on the first success.
  const sessionToken = state.token;
  const userPending = sessionToken !== null && state.user === null;
  useEffect(() => {
    if (!sessionToken) return;
    const interval = setInterval(
      () => {
        authApi
          .getMe(sessionToken)
          .then((user) => setState((s) => (s.token === sessionToken && !s.user ? { ...s, user } : s)))
          // A 401 already fired "auth:expired" inside request(); any other
          // error (network, 5xx, 429) is transient and keeps the session.
          .catch(() => {});
      },
      userPending ? 5_000 : 60_000,
    );
    return () => clearInterval(interval);
  }, [sessionToken, userPending]);

  const login = useCallback(async (username: string, password: string) => {
    const resp = await authApi.login(username, password);
    localStorage.setItem(TOKEN_KEY, resp.token);
    setState({ user: resp.user, token: resp.token, loading: false });
  }, []);

  const updateUser = useCallback((patch: Partial<UserProfile>) => {
    setState((s) => (s.user ? { ...s, user: { ...s.user, ...patch } } : s));
  }, []);

  const logout = useCallback(async () => {
    const token = localStorage.getItem(TOKEN_KEY);
    if (token) {
      await authApi.logout(token).catch(() => {});
      localStorage.removeItem(TOKEN_KEY);
    }
    setState({ user: null, token: null, loading: false });
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      ...state,
      login,
      logout,
      updateUser,
      isAuthenticated: !!state.token,
    }),
    [state, login, logout, updateUser],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
