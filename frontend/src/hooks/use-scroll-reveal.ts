import { useCallback, useRef } from "react";

export function useScrollReveal<T extends HTMLElement = HTMLDivElement>(options: IntersectionObserverInit = {}) {
  const observerRef = useRef<IntersectionObserver | null>(null);

  const ref = useCallback(
    (el: T | null) => {
      if (observerRef.current) {
        observerRef.current.disconnect();
        observerRef.current = null;
      }
      if (!el) return;

      const observer = new IntersectionObserver(
        ([entry]) => {
          if (entry.isIntersecting) {
            el.classList.add("sr-revealed");
            observer.disconnect();
          }
        },
        { threshold: 0, ...options },
      );
      observerRef.current = observer;
      observer.observe(el);
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );

  return ref;
}
