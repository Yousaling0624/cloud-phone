import { createContext, useContext } from "react";
import type { Account, Snapshot } from "../mygo";
import type { PageKey, Tone } from "./types";

export interface AppContextValue {
  snap: Snapshot;
  refresh: () => Promise<void>;
  notify: (message: string, tone?: Tone) => void;
  go: (page: PageKey) => void;
  query: string;
  setQuery: (q: string) => void;
  openAccountForm: (account?: Account) => void;
  openImport: () => void;
  confirmDelete: (ids: number[]) => void;
  openConsole: (accountId: number) => void;
}

export const AppContext = createContext<AppContextValue | null>(null);

export function useApp(): AppContextValue {
  const ctx = useContext(AppContext);
  if (!ctx) throw new Error("useApp must be used inside AppContext");
  return ctx;
}
