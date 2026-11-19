import { useCallback, useEffect, useRef, useState } from "react";
import { API } from "../mygo";
import { errText } from "./format";

const POLL_MS = 3000;
const MAX_BACKOFF_MS = 15000;

/**
 * Polls the bound phone's screenshot. It pauses while the page is hidden,
 * backs off when the provider is failing, and re-polling immediately when
 * the page becomes visible again. Responses are applied in order; switching
 * accounts cancels the old loop.
 */
export function useScreenshot(accountId: number, enabled: boolean) {
  const [src, setSrc] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const seq = useRef(0);
  const kick = useRef<() => void>(() => {});

  const refresh = useCallback(() => {
    kick.current();
  }, []);

  useEffect(() => {
    if (!enabled) {
      setSrc(null);
      setError("");
      setLoading(false);
      return;
    }
    let cancelled = false;
    let timer = 0;
    let delay = 0;
    let wake: (() => void) | null = null;

    const wait = (ms: number) =>
      new Promise<void>((resolve) => {
        wake = resolve;
        timer = window.setTimeout(resolve, ms);
      });

    const loop = async () => {
      while (!cancelled) {
        if (document.hidden) {
          await wait(POLL_MS);
          continue;
        }
        const mine = ++seq.current;
        setLoading((v) => v || src === null);
        try {
          const next = await API.accountScreenshot(accountId);
          if (cancelled || mine !== seq.current) return;
          setSrc(next);
          setError("");
          delay = 0;
        } catch (e) {
          if (cancelled || mine !== seq.current) return;
          setError(errText(e));
          delay = delay === 0 ? POLL_MS : Math.min(delay * 2, MAX_BACKOFF_MS);
        } finally {
          if (!cancelled) setLoading(false);
        }
        await wait(delay || POLL_MS);
      }
    };

    const onVisible = () => {
      if (!document.hidden && !cancelled) {
        delay = 0;
        window.clearTimeout(timer);
        wake?.();
      }
    };
    kick.current = onVisible;
    document.addEventListener("visibilitychange", onVisible);
    void loop();
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
      wake?.();
      document.removeEventListener("visibilitychange", onVisible);
      kick.current = () => {};
    };
    // src is intentionally not a dependency: it only affects the loading flag.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accountId, enabled]);

  return { src, error, loading, refresh };
}
